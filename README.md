# VMware VMs to CNV validation

To run the OPA server with VMware validations there are few options
1) Local opa server
```
git clone git@github.com:kubev2v/forklift.git && cd forklift
opa run --server ./validation/policies/io/konveyor/forklift/vmware
```
2) Official forklift image
```
# Note that we must override the entrypoint as it expects TLS cert.
podman run -p 8181:8181 -d --name opa --entrypoint '/usr/bin/opa' quay.io/kubev2v/forklift-validation run --server /usr/share/opa/policies
```

To run the validation execute:
```
export VSPHERE_SERVER="https://vcenter.example.com/sdk"
export VSPHERE_USERNAME="admin@example.com"
export VSPHERE_PASSWORD"123456"
go run main.go
```

It will generate the validation per VM in the file ```/tmp/output.json```
Path can be overriden by env variable `OUTPUT_FILE`.


### Ouput
Example output:
```
{
  "rhelvm": {
    "result": [
      {
        "assessment": "Changed Block Tracking (CBT) has not been enabled on this VM. This feature is a prerequisite for VM warm migration.",
        "category": "Warning",
        "label": "Changed Block Tracking (CBT) not enabled"
      }
    ]
  },
  "rvanderp-rhcos": {
    "result": [
      {
        "assessment": "Changed Block Tracking (CBT) has not been enabled on this VM. This feature is a prerequisite for VM warm migration.",
        "category": "Warning",
        "label": "Changed Block Tracking (CBT) not enabled"
      }
    ]
  }
```
