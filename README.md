# Ettus Device Plugin for Kubernetes

This code provides a Kubernetes [Device Plugin](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
to manage Ettus USRPs as Kubernetes Node Resources.

Currently tested USRPS are:
- B210.

It should work with:
- B200.
- B210.
- B200Mini.
- B205Mini.

The device plugin detects and registers connected USRPs. It also download uhd_images in node path '/usr/share/uhd/images'.
When a pod request a "ettus.com/usrp" resource, the ettus device plugin automatically attach the corresponding '/dev/bus/usb/' device and mounts '/usr/share/uhd/images' hostPath in the pod.

## Build Binary

```
make build
```

This will generate bin/ettus-device-plugin binary.

## Deployment

You can run ettus-device-plugin manually at each node or with the provided DaemonSet.
You must run it as superuser because the binary must access /dev/bus/serial and create /usr/share/uhd/images to download uhd_images.

You must have ettus device attached prior to run the ettus-device-plugin. We plan to support dynamic attach-dettach of usrps in the future.

After deployment, you can check if USRPs are detected and included as node resources with:
```
kubectl get nodes -o go-template --template='{{range .items}}{{printf "%s %s\n" .metadata.name .status.allocatable}}{{end}}'

kube-node1 map[cpu:8 ephemeral-storage:385306984Ki ettus.com/usrp:1 hugepages-1Gi:0 hugepages-2Mi:0 memory:16003440Ki pods:110]
```

## Deploy as DaemonSet
We also include 'ettus-daemonset.yaml' to deploy ettus-device-plugin as a DaemonSet so you can rely on Kubernetes to: place the device plugin's Pod onto Nodes, to restart the daemon Pod after failure, and to help automate upgrades.

We have not yet published ettus-device-plugin docker images, so you must build the image locally and distribute it to your kubernetes nodes.

To build the docker image:

```
make docker
```

If you are trying a local kubernetes (microk8s or k3s), you can import the local image with:
```
./containerd_image_import.sh [microk8s|k3s] gradiant/ettus-device-plugin:0.0.1
```


## Test a Pod
The following command run a pod asking for an ettus/usrp. The image cgiraldo/ettus-uhd includes uhd libraries and examples (not the uhd images, that are automatically mounted by the ettus device plugin):

```
kubectl run test-usrp -ti --rm --privileged --image cgiraldo/ettus-uhd --limits="ettus.com/usrp=1" -- benchmark_rate --rx_rate 10e6 --tx_rate 10e6
...
Benchmark rate summary:
  Num received samples:     102221483
  Num dropped samples:      0
  Num overruns detected:    0
  Num transmitted samples:  100064040
  Num sequence errors (Tx): 0
  Num sequence errors (Rx): 0
  Num underruns detected:   0
  Num late commands:        0
  Num timeouts (Tx):        2
  Num timeouts (Rx):        0


Done!
```

We can also deploy an openairinterface enodeb (CTRL-C to terminate):
```
kubectl run test-usrp-oai -ti --rm --privileged --image openverso/oai-enb:1.2.2 --limits="ettus.com/usrp=1"
```