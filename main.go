package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-09-01/network"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
)

var (
	subscription = os.Args[1]
	publisher    = "Canonical"
	offer        = "UbuntuServer"
	region       = "eastus"
	clientID     = os.Args[2]
	tenantID     = os.Args[3]
)

func main() {
	fmt.Println(getSkus(context.Background(), region, publisher, offer))
}

func vmClient() compute.VirtualMachinesClient {
	vmClient := compute.NewVirtualMachinesClient(subscription)
	devicecgf := auth.NewDeviceFlowConfig(clientID, tenantID)
	authorizer, err := devicecgf.Authorizer()
	if err == nil {
		vmClient.Authorizer = authorizer
	}
	return vmClient
}

func vnetClient() network.VirtualNetworksClient {
	vclt := network.NewVirtualNetworksClient(subscription)
	devicecgf := auth.NewDeviceFlowConfig(clientID, tenantID)
	authorizer, err := devicecgf.Authorizer()
	if err == nil {
		vclt.Authorizer = authorizer
	}
	return vclt
}

func createVnet(ctx context.Context, rg, vname, region, cidr, sname, subcidr string) {
	client := vnetClient()
	client.CreateOrUpdate(ctx,
		rg,
		vname,
		network.VirtualNetwork{
			Location: to.StringPtr(region),
			VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
				AddressSpace: &network.AddressSpace{
					AddressPrefixes: to.StringSlicePtr([]string{cidr}),
				},
				Subnets: &[]network.Subnet{
					{
						Name: to.StringPtr(sname),
						SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
							AddressPrefix: to.StringPtr(subcidr),
						},
					},
				},
			},
		},
	)
}

func createVM(ctx context.Context, rg, vmname, region string) {
	client := vmClient()
	client.CreateOrUpdate(ctx,
		rg,
		vmname,
		compute.VirtualMachine{
			Location: to.StringPtr(region),
			VirtualMachineProperties: &compute.VirtualMachineProperties{
				HardwareProfile: &compute.HardwareProfile{
					VMSize: compute.VirtualMachineSizeTypesStandardD2sV3,
				},
				StorageProfile: &compute.StorageProfile{
					ImageReference: &compute.ImageReference{
						Publisher: to.StringPtr(publisher),
					},
				},
			},
		},
	)
}

func getSkus(ctx context.Context, region, publisher, offer string) *[]compute.VirtualMachineImageResource {
	client := compute.NewVirtualMachineImagesClient(subscription)
	devicecgf := auth.NewDeviceFlowConfig(clientID, tenantID)
	authorizer, err := devicecgf.Authorizer()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	if err == nil {
		client.Authorizer = authorizer
	}
	result, err := client.ListSkus(ctx, region, publisher, offer)
	if err != nil {
		panic(err)
	}
	return result.Value
}

func getVMimages(ctx context.Context, region, publisher, offer, skus, expand, orderby string, top int32) *[]compute.VirtualMachineImageResource {
	client := compute.NewVirtualMachineImagesClient(subscription)
	devicecgf := auth.NewDeviceFlowConfig(clientID, tenantID)
	authorizer, err := devicecgf.Authorizer()
	if err == nil {
		client.Authorizer = authorizer
	}
	result, err := client.List(ctx, region, publisher, offer, skus, expand, &top, orderby)
	if err != nil {
		panic(err)
	}
	return result.Value
}
