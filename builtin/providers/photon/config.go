package photon

import (
	"fmt"
	"log"

	"github.com/golang/glog"
	"github.com/vmware/photon-controller-go-sdk/photon"
)

type Config struct {
	CloudTarget       string
	IgnoreCertificate bool
	Tenant            string
	Project           string
	OverrideIP        bool
}

// Client() returns a new client for accessing Photon Controller Management Node.
func (c *Config) Client() (*photon.Client, error) {
	if len(c.CloudTarget) == 0 {
		return nil, fmt.Errorf("Photon Controller endpoint was not specified.")
	}

	options := &photon.ClientOptions{
		IgnoreCertificate: c.IgnoreCertificate,
	}

	client := photon.NewClient(c.CloudTarget, options, nil)
	status, err := client.Status.Get()
	if err != nil {
		glog.Errorf("Photon Provider: new client creation failed. Error[%v]", err)
		return nil, err
	}

	log.Printf("[INFO] Photon Controller Client configured for URL: %s, status %v", c.CloudTarget, status)

	return client, nil
}
