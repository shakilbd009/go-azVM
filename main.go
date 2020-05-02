package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-09-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
)

var (
	region       = "eastus"
	avSku        = "aligned"
	publisher    = "Canonical"
	offer        = "UbuntuServer"
	vnetcidr     = "10.0.0.0/16"
	subnetcidr   = "10.0.0.0/24"
	RGname       = "az-nonProd-rg-006"
	VnetName     = "az-nonProd-vnet-001"
	subnetName   = "az-nonProd-sub-001"
	AVsetName    = "az-nonProd-avs-001"
	NSGname      = "az-nonProd-nsg-001"
	rdesc        = "az-nonProd-rule-001"
	subscription = os.Args[1]
	priority     = 100
	username     = "usertest"
	passwd       = "useRword123$"
	vmname       = "azxeptst01"
)

func main() {
	start := time.Now()
	skch := make(chan *[]compute.VirtualMachineImageResource)
	imch := make(chan *[]compute.VirtualMachineImageResource)
	vnch := make(chan string)
	rgch := make(chan resources.Group)
	avch := make(chan string)
	vgch := make(chan network.VirtualNetwork)
	ngch := make(chan string)
	sbch := make(chan string)
	vmch := make(chan string)
	nich := make(chan string)
	ctx := context.Background()
	nic := fmt.Sprintf("%s-nic-%v", vmname, 01)
	go getSkus(ctx, region, publisher, offer, skch)
	go createRG(ctx, RGname, region, rgch)
	skus := *<-skch
	sknm := ""
	igvs := ""
	for _, v := range skus[len(skus)-1:] {

		go getVMimages(ctx, region, publisher, offer, *v.Name, imch)
		sknm = *v.Name
		igvs = *(*<-imch)[0].Name
	}
	rg := <-rgch
	go createVnet(ctx, *rg.Name, VnetName, region, vnetcidr, subnetName, subnetcidr, vnch)
	go createAVS(ctx, AVsetName, *rg.Name, avSku, region, avch)
	go getVnet(ctx, *rg.Name, VnetName, vgch)
	fmt.Println(<-vnch)
	vname := <-vgch
	go createSubnet(ctx, *rg.Name, subnetName, *vname.Name, subnetcidr, sbch)
	subid := <-sbch
	go createNSG(ctx, *rg.Name, NSGname, subscription, rdesc, subid, region, int32(priority), ngch)
	go createNIC(ctx, *rg.Name, nic, subscription, region, <-ngch, subid, nich)
	go createVM(ctx, *rg.Name, vmname, username, passwd, <-nich, <-avch, region, publisher, offer, sknm, igvs, vmch)
	for {
		time.Sleep(time.Millisecond * 1000)
		select {
		case vm := <-vmch:
			fmt.Println(sknm, igvs, vm)
			end := time.Now().Sub(start).Seconds()
			fmt.Printf("time took %.2f seconds\n", end)
			return
		default:
			log.Println("Deploying VM...")
		}
	}
}

func checkRG(ctx context.Context, name, subscription string) autorest.Response {
	client := resources.NewGroupsClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		client.Authorizer = authorizer
	}
	defer errRecover()
	resp, err := client.CheckExistence(ctx, name)
	if err != nil {
		panic(err)
	}
	return resp
}

func createRG(ctx context.Context, name, loc string, ch chan resources.Group) {
	client := resources.NewGroupsClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		client.Authorizer = authorizer
	}
	defer errRecover()
	resp, err := client.CreateOrUpdate(ctx,
		name,
		resources.Group{
			Name:     to.StringPtr(name),
			Location: to.StringPtr(loc),
		},
	)
	if err != nil {
		panic(err)
	}
	ch <- resp
	close(ch)
}

func getVMimages(ctx context.Context, region, publisher, offer, skus string, ch chan *[]compute.VirtualMachineImageResource) {
	client := compute.NewVirtualMachineImagesClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		client.Authorizer = authorizer
	}
	defer errRecover()
	result, err := client.List(ctx, region, publisher, offer, skus, "", nil, "")
	if err != nil {
		panic(err)
	}
	ch <- result.Value
	close(ch)
}

func vmClient() compute.VirtualMachinesClient {
	vmClient := compute.NewVirtualMachinesClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		vmClient.Authorizer = authorizer
	}
	return vmClient
}

func vnetClient() network.VirtualNetworksClient {
	vclt := network.NewVirtualNetworksClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		vclt.Authorizer = authorizer
	}
	return vclt
}

func getVnet(ctx context.Context, rg, vname string, ch chan network.VirtualNetwork) {
	vclt := network.NewVirtualNetworksClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		vclt.Authorizer = authorizer
	}
	defer errRecover()
	time.Sleep(time.Second * 2)
	resp, err := vclt.Get(ctx, rg, vname, "")
	if err != nil {
		panic(err)
	}
	ch <- resp
	close(ch)
}

func createVnet(ctx context.Context, rg, vname, region, cidr, sname, subcidr string, ch chan string) {
	client := vnetClient()
	defer errRecover()
	resp, err := client.CreateOrUpdate(ctx,
		rg,
		vname,
		network.VirtualNetwork{
			Location: to.StringPtr(region),
			VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
				AddressSpace: &network.AddressSpace{
					AddressPrefixes: to.StringSlicePtr([]string{cidr}),
				},
			},
		},
	)
	time.Sleep(2 * time.Second)
	if err = resp.WaitForCompletionRef(ctx, client.Client); err == nil {
		inter, err := resp.Result(client)
		if err != nil {
			panic(err)
		}
		ch <- *inter.ID
		close(ch)
	}

}

func createVM(ctx context.Context, rg, vmname, username, passwd, nic, avsID, region, publisher, offer, sku, version string, ch chan string) {
	client := vmClient()
	defer errRecover()
	resp, err := client.CreateOrUpdate(ctx,
		rg,
		vmname,
		compute.VirtualMachine{
			Location: to.StringPtr(region),
			VirtualMachineProperties: &compute.VirtualMachineProperties{
				HardwareProfile: &compute.HardwareProfile{
					VMSize: compute.VirtualMachineSizeTypesStandardD2sV3,
				},
				StorageProfile: &compute.StorageProfile{
					DataDisks: &[]compute.DataDisk{
						{
							Lun:          to.Int32Ptr(0),
							Name:         to.StringPtr(vmname + "01"),
							Caching:      compute.CachingTypesReadWrite,
							CreateOption: compute.DiskCreateOptionTypesEmpty,
							DiskSizeGB:   to.Int32Ptr(40),
							ManagedDisk: &compute.ManagedDiskParameters{
								StorageAccountType: compute.StorageAccountTypesStandardLRS,
							},
						},
					},
					ImageReference: &compute.ImageReference{
						Publisher: to.StringPtr(publisher),
						Offer:     to.StringPtr(offer),
						Sku:       to.StringPtr(sku),
						Version:   to.StringPtr(version),
					},
				},
				OsProfile: &compute.OSProfile{
					ComputerName:  to.StringPtr(vmname),
					AdminUsername: to.StringPtr(username),
					AdminPassword: to.StringPtr(passwd),
				},
				NetworkProfile: &compute.NetworkProfile{
					NetworkInterfaces: &[]compute.NetworkInterfaceReference{
						{
							ID: to.StringPtr(nic),
						},
					},
				},
				AvailabilitySet: &compute.SubResource{
					ID: to.StringPtr(avsID),
				},
			},
		},
	)
	if err != nil {
		panic(err)
	}
	time.Sleep(time.Second * 60)
	inter, err := resp.Result(client)
	if err != nil {
		panic(err)
	}
	ch <- *&inter.Status
	close(ch)

}

func getSkus(ctx context.Context, region, publisher, offer string, ch chan *[]compute.VirtualMachineImageResource) {
	client := compute.NewVirtualMachineImagesClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		client.Authorizer = authorizer
	}
	defer errRecover()
	result, err := client.ListSkus(ctx, region, publisher, offer)
	if err != nil {
		panic(err)
	}
	ch <- result.Value
	close(ch)
}

func createAVS(ctx context.Context, name, rg, sku, loc string, ch chan string) {
	client := compute.NewAvailabilitySetsClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		client.Authorizer = authorizer
	}
	defer errRecover()
	avSet, err := client.CreateOrUpdate(ctx,
		rg,
		name,
		compute.AvailabilitySet{
			AvailabilitySetProperties: &compute.AvailabilitySetProperties{
				PlatformFaultDomainCount:  to.Int32Ptr(2),
				PlatformUpdateDomainCount: to.Int32Ptr(5),
			},
			Sku: &compute.Sku{
				Name: to.StringPtr(sku),
			},
			Name:     to.StringPtr(name),
			Location: to.StringPtr(loc),
		},
	)
	if err != nil {
		panic(err)
	}
	ch <- *avSet.ID
	close(ch)
}

func createNIC(ctx context.Context, rg, nicname, subscription, loc, nsgID, subid string, ch chan string) {
	client := network.NewInterfacesClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		client.Authorizer = authorizer
	}
	defer errRecover()
	resp, err := client.CreateOrUpdate(ctx,
		rg,
		nicname,
		network.Interface{
			Location: to.StringPtr(loc),
			InterfacePropertiesFormat: &network.InterfacePropertiesFormat{
				NetworkSecurityGroup: &network.SecurityGroup{
					ID: to.StringPtr(nsgID),
				},
				IPConfigurations: &[]network.InterfaceIPConfiguration{
					{
						Name: to.StringPtr("ipConfig"),
						InterfaceIPConfigurationPropertiesFormat: &network.InterfaceIPConfigurationPropertiesFormat{
							PrivateIPAllocationMethod: network.Dynamic,
							PrivateIPAddressVersion:   network.IPv4,
							Subnet: &network.Subnet{
								ID: to.StringPtr(subid),
							},
						},
					},
				},
			},
		},
	)
	inter, err := resp.Result(client)
	if err != nil {
		panic(err)
	}
	ch <- *inter.ID
	close(ch)
}

func createNSG(ctx context.Context, rg, name, subscription, ruledesc, subnet, region string, priority int32, ch chan string) {
	client := network.NewSecurityGroupsClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		client.Authorizer = authorizer
	}
	defer errRecover()
	resp, err := client.CreateOrUpdate(ctx, rg, name,
		network.SecurityGroup{
			Location: to.StringPtr(region),
			Name:     to.StringPtr(name),
			SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
				SecurityRules: &[]network.SecurityRule{
					{Name: to.StringPtr("allow-to-any-port"),
						SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
							Description:              to.StringPtr(ruledesc),
							Protocol:                 network.SecurityRuleProtocolTCP,
							SourceAddressPrefixes:    to.StringSlicePtr([]string{"172.25.78.0/24"}),
							SourcePortRange:          to.StringPtr("*"),
							DestinationAddressPrefix: to.StringPtr("*"),
							DestinationPortRange:     to.StringPtr("*"),
							Priority:                 to.Int32Ptr(priority),
							Access:                   network.SecurityRuleAccessAllow,
							Direction:                network.SecurityRuleDirectionInbound,
						},
					},
				},
				Subnets: &[]network.Subnet{
					{
						ID: to.StringPtr(subnet),
					},
				},
			},
		},
	)
	inter, err := resp.Result(client)
	if err != nil {
		panic(err)
	}
	ch <- *inter.ID
	close(ch)
}

func getSubnet(ctx context.Context, rg, sname, vname string, ch chan string) {
	client := network.NewSubnetsClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	defer errRecover()
	if err == nil {
		client.Authorizer = authorizer
	}
	resp, err := client.Get(ctx, rg, vname, sname, "")
	if err != nil {
		panic(err)
	}
	ch <- *resp.ID
	close(ch)
}

func createSubnet(ctx context.Context, rg, sname, vname, cidr string, ch chan string) {
	defer errRecover()
	client := network.NewSubnetsClient(subscription)
	authorizer, err := auth.NewAuthorizerFromCLI()
	if err == nil {
		client.Authorizer = authorizer
	}
	resp, err := client.CreateOrUpdate(ctx, rg, vname, sname, network.Subnet{
		SubnetPropertiesFormat: &network.SubnetPropertiesFormat{
			AddressPrefix: to.StringPtr(cidr),
		},
	})
	time.Sleep(time.Second * 2)
	inter, err := resp.Result(client)
	if err != nil {
		panic(err)
	}
	ch <- *inter.ID
	close(ch)
}

func errRecover() {
	if r := recover(); r != nil {
		fmt.Println("An error has occured:")
		fmt.Printf("%s\n\n", strings.Repeat("💀", 20))
		fmt.Println(r)
		fmt.Printf("\n")
		fmt.Println(strings.Repeat("💀", 20))
		os.Exit(1) //optional, if you want to stop the excution if error occurs.
	}
}
