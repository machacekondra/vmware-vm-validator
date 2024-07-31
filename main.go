package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	api "github.com/konveyor/forklift-controller/pkg/apis/forklift/v1beta1"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/container/vsphere"
	"github.com/konveyor/forklift-controller/pkg/controller/provider/model"
	vspheremodel "github.com/konveyor/forklift-controller/pkg/controller/provider/model/vsphere"
	web "github.com/konveyor/forklift-controller/pkg/controller/provider/web/vsphere"
	libmodel "github.com/konveyor/forklift-controller/pkg/lib/inventory/model"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
	server := os.Getenv("VSPHERE_SERVER")
	username := os.Getenv("VSPHERE_USERNAME")
	password := os.Getenv("VSPHERE_PASSWORD")
	opaServer := os.Getenv("OPA_SERVER")
	if opaServer == "" {
		opaServer = "127.0.0.1:8181"
	}
	outputFile := os.Getenv("OUTPUT_FILE")
	if outputFile == "" {
		outputFile = "/tmp/output.json"
	}

	// Provider
	vsphereType := api.VSphere
	provider := &api.Provider{
		Spec: api.ProviderSpec{
			URL:  server,
			Type: &vsphereType,
		},
	}

	// Secret
	secret := &core.Secret{
		ObjectMeta: meta.ObjectMeta{
			Name:      "vsphere-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"user":               []byte(username),
			"password":           []byte(password),
			"insecureSkipVerify": []byte("true"),
		},
	}

	// Check if opaServer is responding
	resp, err := http.Get("http://" + opaServer + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Println("OPA server " + opaServer + " is not responding")
		return
	}
	defer resp.Body.Close()

	// DB
	path := filepath.Join("/tmp", "db.db")
	models := model.Models(provider)
	db := libmodel.New(path, models...)
	err = db.Open(true)
	if err != nil {
		fmt.Println(err)
		return
	}

	// Vshere collector
	collector := vsphere.New(db, provider, secret)
	defer collector.Shutdown()

	// Collect
	err = collector.Start()
	if err != nil {
		fmt.Println(err)
		return
	}

	// Wait for collector.
	for {
		time.Sleep(1 * time.Second)
		if collector.HasParity() {
			break
		}
	}

	// List VMs
	vms := &[]vspheremodel.VM{}
	err = collector.DB().List(vms, libmodel.FilterOptions{})
	if err != nil {
		fmt.Println(err)
		return
	}

	vmReport := make(map[string]interface{})
	for _, vm := range *vms {
		// Prepare the JSON data to MTV OPA server format.
		r := web.Workload{}
		r.With(&vm)
		vmJson := map[string]interface{}{
			"input": r,
		}

		vmData, err := json.Marshal(vmJson)
		if err != nil {
			fmt.Println(err)
			return
		}

		// Prepare the HTTP request to OPA server
		req, err := http.NewRequest(
			"POST",
			fmt.Sprintf("http://%s/v1/data/io/konveyor/forklift/vmware/concerns", opaServer),
			bytes.NewBuffer(vmData),
		)
		if err != nil {
			fmt.Println("Error creating HTTP request:", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		// Send the HTTP request to OPA server
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("Error sending HTTP request:", err)
			vmReport[vm.Name] = map[string]interface{}{"failed": true}
			continue
		}

		// Check the response status
		if resp.StatusCode != http.StatusOK {
			fmt.Printf("Received non-OK response: %s\n", resp.Status)
			vmReport[vm.Name] = map[string]interface{}{"failed": true}
			return
		}

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error reading response body:", err)
			vmReport[vm.Name] = map[string]interface{}{"failed": true}
			return
		}

		// Save the report to map
		var responseMap map[string]interface{}
		err = json.Unmarshal(body, &responseMap)
		if err != nil {
			fmt.Println("Error unmarshalling response body:", err)
			vmReport[vm.Name] = map[string]interface{}{"failed": true}
			return
		}
		vmReport[vm.Name] = responseMap
		resp.Body.Close()
	}

	// Create or open the file
	file, err := os.Create(outputFile)
	if err != nil {
		fmt.Println("Error creating file:", err)
		return
	}
	// Ensure the file is closed properly
	defer file.Close()

	// Write the string to the file
	jsonData, err := json.Marshal(vmReport)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return
	}
	_, err = file.Write(jsonData)
	if err != nil {
		fmt.Println("Error writing to file:", err)
		return
	}
}
