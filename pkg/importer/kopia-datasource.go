package importer

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
	"net/url"
	"os/exec"
	"strings"
)

// KopiaDataSource is the struct containing the information needed to import from an kopia data source.
// Sequence of phases:
// 1. Info -> Transfer
// 2. Transfer -> Convert
type KopiaDataSource struct {
	// S3 end point
	ep *url.URL
	// User name
	accessKey string
	// Password
	secKey string
}

// Info is called to get initial information about the data.
func (k *KopiaDataSource) Info() (ProcessingPhase, error) {
	klog.Infof("ep: %s", k.ep)
	klog.Infof("endpoint: %s", k.ep.Host)
	queryParams := k.ep.Query()
	for key, values := range queryParams {
		klog.Infof("%s: %v\n", key, values)
	}
	klog.Infof("acc: %s, sec: %s", k.accessKey, k.secKey)

	var err error

	err = kopiaRepoConnect("s3", queryParams.Get("bucket"), k.accessKey, k.secKey, k.ep.Host, queryParams.Get("prefix"))
	if err != nil {
		klog.Errorf("kopia repo connect error: %s", err.Error())
	} else {
		klog.Infof("kopia repo connected.")
	}

	err = kopiaRepoStatus()
	if err != nil {
		klog.Errorf("kopia repo status error: %s", err.Error())
	} else {
		klog.Infof("kopia repo status:")
	}

	return ProcessingPhaseTransferDataFile, err
}

func (k *KopiaDataSource) Transfer(path string) (ProcessingPhase, error) {
	return ProcessingPhaseError, errors.New("Transfer should not be called")
}

func (k *KopiaDataSource) TransferFile(fileName string) (ProcessingPhase, error) {
	if err := CleanAll(fileName); err != nil {
		return ProcessingPhaseError, err
	}
	klog.Infof("fileName: %s", fileName)

	if err := kopiaRestoreSnapshot(k.ep.Query().Get("snapshot"), fileName); err != nil {
		klog.Errorf("kopiaRestoreSnapshot error: %s", err.Error())
	}
	return ProcessingPhaseResize, nil
}

func (k *KopiaDataSource) GetURL() *url.URL {
	return k.ep
}

func (k *KopiaDataSource) Close() error {
	return nil
}

func NewKopiaDataSource(endpoint, accessKey, secKey string) (*KopiaDataSource, error) {
	ep, err := ParseEndpoint(endpoint)
	if err != nil {
		return nil, errors.Wrapf(err, fmt.Sprintf("unable to parse endpoint %q", endpoint))
	}
	return &KopiaDataSource{
		ep:        ep,
		accessKey: accessKey,
		secKey:    secKey,
	}, nil
}

func kopiaRepoConnect(storageType, bucketName, acc, sec, endpoint, prefix string) error {
	return kopiaCmd("repo", "connect", storageType,
		fmt.Sprintf("--bucket=%s", bucketName), fmt.Sprintf("--access-key=%s", acc),
		fmt.Sprintf("--secret-access-key=%s", sec), fmt.Sprintf("--endpoint=%s", endpoint),
		fmt.Sprintf("--prefix=%s", prefix), "-p static-passw0rd")
}

func kopiaRepoStatus() error {
	return kopiaCmd("repo", "status")
}

func kopiaRestoreSnapshot(snapshotID, fileName string) error {
	return kopiaCmd("restore", snapshotID+"/disk.img", fileName)
}

func kopiaCmd(args ...string) error {
	cmd := exec.Command("kopia", args...)
	klog.Infof("kopia cmd: %s", cmd.Args)

	var out bytes.Buffer
	cmd.Stdout = &out

	var outErr bytes.Buffer
	cmd.Stderr = &outErr

	err := cmd.Run()

	output := strings.TrimSpace(out.String())
	klog.Infof("kopia cmd stdout: %s", output)

	outputErr := strings.TrimSpace(outErr.String())
	klog.Infof("kopia cmd stderr: %s", outputErr)

	if err != nil {
		klog.Errorf("Error running command: %s", err.Error())
		return err
	}

	return nil
}
