package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	piperDocker "github.com/SAP/jenkins-library/pkg/docker"
	"github.com/SAP/jenkins-library/pkg/log"
	"github.com/SAP/jenkins-library/pkg/piperutils"
	"github.com/SAP/jenkins-library/pkg/telemetry"

	"github.com/pkg/errors"
)

func containerSaveImage(config containerSaveImageOptions, telemetryData *telemetry.CustomData) {
	var cachePath = "./cache"

	dClientOptions := piperDocker.ClientOptions{ImageName: config.ContainerImage, RegistryURL: config.ContainerRegistryURL, LocalPath: config.FilePath, IncludeLayers: config.IncludeLayers}
	dClient := &piperDocker.Client{}
	dClient.SetOptions(dClientOptions)

	_, err := runContainerSaveImage(&config, telemetryData, cachePath, "", dClient, piperutils.Files{})
	if err != nil {
		log.Entry().WithError(err).Fatal("step execution failed")
	}
}

func runContainerSaveImage(config *containerSaveImageOptions, telemetryData *telemetry.CustomData, cachePath, rootPath string, dClient piperDocker.Download, fileUtils piperutils.FileUtils) (string, error) {
	if err := correctContainerDockerConfigEnvVar(config, fileUtils); err != nil {
		return "", err
	}

	err := os.RemoveAll(cachePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to prepare cache")
	}

	err = os.Mkdir(cachePath, 0755)
	if err != nil {
		return "", errors.Wrap(err, "failed to create cache")
	}

	// ensure that download cache is cleaned up at the end
	defer os.RemoveAll(cachePath)

	imageSource, err := dClient.GetImageSource()
	if err != nil {
		return "", errors.Wrap(err, "failed to get docker image source")
	}
	image, err := dClient.DownloadImageToPath(imageSource, cachePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to download docker image")
	}

	tarfilePath := config.FilePath
	if len(tarfilePath) == 0 {
		tarfilePath = filenameFromContainer(rootPath, config.ContainerImage)
	} else {
		tarfilePath = filenameFromContainer(rootPath, tarfilePath)
	}

	tarFile, err := os.Create(tarfilePath)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create %v for docker image", tarfilePath)
	}
	defer tarFile.Close()

	if err := os.Chmod(tarfilePath, 0644); err != nil {
		return "", errors.Wrapf(err, "failed to adapt permissions on %v", tarfilePath)
	}

	err = dClient.TarImage(tarFile, image)
	if err != nil {
		return "", errors.Wrap(err, "failed to tar container image")
	}

	return tarfilePath, nil
}

func filenameFromContainer(rootPath, containerImage string) string {
	return filepath.Join(rootPath, strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(containerImage, "/", "_"), ":", "_"), ".", "_")+".tar")
}

func correctContainerDockerConfigEnvVar(config *containerSaveImageOptions, utils piperutils.FileUtils) error {
	dockerConfigDir, err := utils.TempDir("", "docker")

	if err != nil {
		return errors.Wrap(err, "unable to create docker config dir")
	}

	dockerConfigFile := fmt.Sprintf("%s/%s", dockerConfigDir, "config.json")

	if len(config.DockerConfigJSON) > 0 {
		log.Entry().Infof("Docker credentials configuration: %v", config.DockerConfigJSON)

		if exists, _ := utils.FileExists(config.DockerConfigJSON); exists {
			if _, err = utils.Copy(config.DockerConfigJSON, dockerConfigFile); err != nil {
				return errors.Wrap(err, "unable to copy docker config")
			}
		}
	} else {
		log.Entry().Info("Docker credentials configuration: NONE")
	}

	if len(config.ContainerRegistryURL) > 0 && len(config.ContainerRegistryUser) > 0 && len(config.ContainerRegistryPassword) > 0 {
		if _, err = piperDocker.CreateDockerConfigJSON(config.ContainerRegistryURL, config.ContainerRegistryUser, config.ContainerRegistryPassword, dockerConfigFile, dockerConfigFile, utils); err != nil {
			log.Entry().Warningf("failed to update Docker config.json: %v", err)
		}
	}

	os.Setenv("DOCKER_CONFIG", dockerConfigDir)

	return nil
}
