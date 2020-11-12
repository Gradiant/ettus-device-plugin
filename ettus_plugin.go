// Copyright 2020 Gradiant
// Author: Carlos Giraldo(cgiraldo@gradiant.org)
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	EttusVendorID     = "2500"
	EttusNiVendorID   = "3923"
	B200ProductID     = "0020"
	B200MiniProductID = "0021"
	B205MiniProductID = "0022"
	B200NiProductID   = "7813"
	B210NiProductID   = "7814"
)
const (
	SysfsDevices = "/sys/bus/usb/devices"
	VendorFile   = "idVendor"
	ProductFile  = "idProduct"
	DeviceFile   = "device"
)

const (
	socketName   string = "ettusUSRP"
	resourceName string = "ettus.com/usrp"
)

type ettusDevice struct {
	vid     string
	pid     string
	name    string
	devPath string
	devNum  string
	device  pluginapi.Device
}

// etttusManager manages ettus devices
type ettusManager struct {
	devices map[string]*ettusDevice
}

func NewEttusManager() (*ettusManager, error) {
	return &ettusManager{
		devices: make(map[string]*ettusDevice),
	}, nil
}

func GetFileContent(file string) (string, error) {
	if buf, err := ioutil.ReadFile(file); err != nil {
		return "", fmt.Errorf("Can't read file %s", file)
	} else {
		ret := strings.Trim(string(buf), "\n")
		return ret, nil
	}
}

func (ettus *ettusManager) discoverEttusResources() (bool, error) {
	found := false
	ettus.devices = make(map[string]*ettusDevice)
	glog.Info("discoverEttusResources")

	usbFiles, err := ioutil.ReadDir(SysfsDevices)
	if err != nil {
		return false, fmt.Errorf("Can't read folder %s", SysfsDevices)
	}

	for _, usbFile := range usbFiles {
		usbID := usbFile.Name()
		if strings.Contains(usbID, ":") {
			continue
		}
		fname := path.Join(SysfsDevices, usbID, VendorFile)
		vendorID, err := GetFileContent(fname)
		if err != nil {
			return false, err
		}
		fname = path.Join(SysfsDevices, usbID, ProductFile)
		productID, err := GetFileContent(fname)
		if err != nil {
			return false, err
		}
		productName := "Undefined"
		if strings.EqualFold(vendorID, EttusVendorID) {
			switch productID {
			case B200ProductID:
				productName = "B200"
			case B200MiniProductID:
				productName = "B200Mini"
			case B205MiniProductID:
				productName = "B205Mini"
			default:
				continue
			}
		} else if strings.EqualFold(vendorID, EttusNiVendorID) {
			switch productID {
			case B200NiProductID:
				productName = "B200"
			case B210NiProductID:
				productName = "B210"
			default:
				continue
			} 
		} else {
			continue
		}
		fname = path.Join(SysfsDevices, usbID, "devpath")
		devpath, err := GetFileContent(fname)
		if err != nil {
			return false, err
		}
		fname = path.Join(SysfsDevices, usbID, "devnum")
		devnum, err := GetFileContent(fname)
		if err != nil {
			return false, err
		}
		fname = path.Join(SysfsDevices, usbID, "serial")
		serial, err := GetFileContent(fname)
		if err != nil {
			return false, err
		}
		healthy := pluginapi.Healthy
		dev := ettusDevice{
			vid:     vendorID,
			pid:     productID,
			name:    productName,
			devPath: fmt.Sprintf("%03s", devpath),
			devNum:  fmt.Sprintf("%03s", devnum),
			device: pluginapi.Device{
				ID:     serial,
				Health: healthy},
		}
		ettus.devices[serial] = &dev
		found = true
	}
	fmt.Printf("Devices: %v \n", ettus.devices)
	return found, nil
}

func (ettus *ettusManager) DownloadUhdImages() error {
	var out bytes.Buffer
	var stderr bytes.Buffer

	fmt.Println("Downloading uhd_images. Be patient")

	cmd := exec.Command("uhd_images_downloader")
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error: CMD uhd_images_downloader: " + fmt.Sprint(err) + ": " + stderr.String())
	}
	return err
}

func (ettus *ettusManager) Init() error {
	glog.Info("Init ettus Manager\n")
	err := ettus.DownloadUhdImages()
	return err
}

func Register(kubeletEndpoint string, pluginEndpoint, socketName string) error {
	conn, err := grpc.Dial(kubeletEndpoint, grpc.WithInsecure(),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}))
	defer conn.Close()
	if err != nil {
		return fmt.Errorf("device-plugin: cannot connect to kubelet service: %v", err)
	}
	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     pluginEndpoint,
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return fmt.Errorf("device-plugin: cannot register to kubelet service: %v", err)
	}
	return nil
}

// Implements DevicePlugin service functions
func (ettus *ettusManager) ListAndWatch(emtpy *pluginapi.Empty, stream pluginapi.DevicePlugin_ListAndWatchServer) error {
	glog.Info("device-plugin: ListAndWatch start\n")
	for {
		ettus.discoverEttusResources()
		resp := new(pluginapi.ListAndWatchResponse)
		for _, dev := range ettus.devices {
			glog.Info("dev ", dev)
			resp.Devices = append(resp.Devices, &dev.device)
		}
		glog.Info("resp.Devices ", resp.Devices)
		if err := stream.Send(resp); err != nil {
			glog.Errorf("Failed to send response to kubelet: %v\n", err)
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (ettus *ettusManager) Allocate(ctx context.Context, rqt *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	glog.Info("Allocate")
	resp := new(pluginapi.AllocateResponse)
	for _, containerRqt := range rqt.ContainerRequests {
		containerResp := new(pluginapi.ContainerAllocateResponse)
		resp.ContainerResponses = append(resp.ContainerResponses, containerResp)
		for _, id := range containerRqt.DevicesIDs {
			if dev, ok := ettus.devices[id]; ok {
				devPath := path.Join("/dev/bus/usb/", dev.devPath, dev.devNum)
				containerResp.Devices = append(containerResp.Devices, &pluginapi.DeviceSpec{
					HostPath:      devPath,
					ContainerPath: devPath,
					Permissions:   "mrw",
				})
				containerResp.Mounts = append(containerResp.Mounts, &pluginapi.Mount{
					HostPath:      "/usr/share/uhd/",
					ContainerPath: "/usr/share/uhd/",
					ReadOnly:      true,
				})
			}
			glog.Info("Allocated interface ", id)
		}
	}
	return resp, nil
}

func (ettus *ettusManager) GetPreferredAllocation(ctx context.Context, rqt *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return new(pluginapi.PreferredAllocationResponse), nil
}

func (ettus *ettusManager) PreStartContainer(ctx context.Context, rqt *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return nil, fmt.Errorf("PreStartContainer() should not be called")
}

func (ettus *ettusManager) GetDevicePluginOptions(ctx context.Context, empty *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	fmt.Println("GetDevicePluginOptions: return empty options")
	return new(pluginapi.DevicePluginOptions), nil
}

func main() {
	flag.Parse()
	fmt.Printf("Starting main \n")

	flag.Lookup("logtostderr").Value.Set("true")

	ettus, err := NewEttusManager()
	if err != nil {
		glog.Fatal(err)
		os.Exit(1)
	}

	found, err := ettus.discoverEttusResources()
	if err != nil {
		glog.Fatal(err)
		os.Exit(1)
	}
	if !found {
		// clean up any exisiting device plugin software
		//sfc.UnInit()
		glog.Errorf("No Ettus are present\n")
		os.Exit(1)
	}

	err = ettus.Init()
	if err != nil {
		glog.Errorf("Error downloading uhd images")
	}

	pluginEndpoint := fmt.Sprintf("%s-%d.sock", socketName, time.Now().Unix())
	var wg sync.WaitGroup
	wg.Add(1)
	// Starts device plugin service.
	go func() {
		defer wg.Done()
		fmt.Printf("DveicePluginPath %s, pluginEndpoint %s\n", pluginapi.DevicePluginPath, pluginEndpoint)
		fmt.Printf("device-plugin start server at: %s\n", path.Join(pluginapi.DevicePluginPath, pluginEndpoint))
		lis, err := net.Listen("unix", path.Join(pluginapi.DevicePluginPath, pluginEndpoint))
		if err != nil {
			glog.Fatal(err)
			return
		}
		grpcServer := grpc.NewServer()
		pluginapi.RegisterDevicePluginServer(grpcServer, ettus)
		grpcServer.Serve(lis)
	}()

	time.Sleep(5 * time.Second)
	// Registers with Kubelet.
	err = Register(pluginapi.KubeletSocket, pluginEndpoint, resourceName)
	if err != nil {
		glog.Fatal(err)
	}
	fmt.Printf("device-plugin registered\n")
	wg.Wait()
}
