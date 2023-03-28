package main

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgTypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2Types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"log"
	"regexp"
)

func CountInstances(ctx context.Context, ec2Client ec2.DescribeInstancesAPIClient, archType ec2Types.ArchitectureType) (int, error) {
	archFilter := ec2Types.Filter{
		Name:   aws.String("architecture"),
		Values: []string{string(archType)},
	}
	statusFilter := ec2Types.Filter{
		Name:   aws.String("instance-state-name"),
		Values: []string{"running"},
	}
	var nextToken *string
	counter := 0
	for {
		req := &ec2.DescribeInstancesInput{
			Filters:   []ec2Types.Filter{archFilter, statusFilter},
			NextToken: nextToken,
		}
		result, err := ec2Client.DescribeInstances(ctx, req)
		if err != nil {
			return counter, err
		}
		counter += len(result.Reservations)
		if result.NextToken != nil {
			nextToken = result.NextToken
		} else {
			break
		}
	}

	return counter, nil
}

func InspectLaunchConfigs(ctx context.Context, asgClient *autoscaling.Client) (*[]asgTypes.LaunchConfiguration, error) {
	var nextToken *string
	configsList := make([]asgTypes.LaunchConfiguration, 0)

	for {
		req := &autoscaling.DescribeLaunchConfigurationsInput{
			NextToken: nextToken,
		}
		result, err := asgClient.DescribeLaunchConfigurations(ctx, req)
		if err != nil {
			return nil, err
		}
		configsList = append(configsList, result.LaunchConfigurations...)
		if result.NextToken != nil {
			nextToken = result.NextToken
		} else {
			break
		}
	}
	return &configsList, nil
}

func isInstanceArm(instanceType string) bool {
	checker := regexp.MustCompile("^\\w+\\d+\\w*g\\w*\\..*$")
	return checker.MatchString(instanceType)
}

func FindLaunchConfigsByArch(allLaunchConfigs *[]asgTypes.LaunchConfiguration, isArm bool) *[]asgTypes.LaunchConfiguration {
	out := make([]asgTypes.LaunchConfiguration, 0)

	for _, lc := range *allLaunchConfigs {
		if isArm {
			if isInstanceArm(*lc.InstanceType) {
				out = append(out, lc)
			}
		} else {
			if !isInstanceArm(*lc.InstanceType) {
				out = append(out, lc)
			}
		}
	}
	return &out
}

func dumpAppIdentityForLC(configs *[]asgTypes.LaunchConfiguration) {
	for _, lc := range *configs {
		log.Printf("\t%s [%s]", *lc.LaunchConfigurationName, *lc.InstanceType)
	}
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("unable to initialise AWS SDK: %v", err)
	}

	ec2Client := ec2.NewFromConfig(cfg)
	asgClient := autoscaling.NewFromConfig(cfg)

	armInstances, err := CountInstances(context.Background(), ec2Client, ec2Types.ArchitectureTypeArm64)
	if err != nil {
		log.Fatal(err)
	}
	x86Instances, err := CountInstances(context.Background(), ec2Client, ec2Types.ArchitectureTypeX8664)
	if err != nil {
		log.Fatal(err)
	}

	pct := float64(armInstances) / float64(armInstances+x86Instances)

	log.Printf("We are currently running %d ARM instances and %d x86 instances.  That is a switchover percentage of %.0f%%", armInstances, x86Instances, pct*100)

	launchConfigs, err := InspectLaunchConfigs(context.Background(), asgClient)
	if err != nil {
		log.Fatal(err)
	}

	armLaunchConfigs := FindLaunchConfigsByArch(launchConfigs, true)
	x86LaunchConfigs := FindLaunchConfigsByArch(launchConfigs, false)
	lcPct := float64(len(*armLaunchConfigs)) / float64(len(*armLaunchConfigs)+len(*x86LaunchConfigs))
	log.Printf("We currently have %d ARM launch configs and %d x86 launch configs.  That is a switchover percentage of %.0f%%", len(*armLaunchConfigs), len(*x86LaunchConfigs), lcPct*100)

	log.Printf("Remaining x86 launch configs are for these apps:")
	dumpAppIdentityForLC(x86LaunchConfigs)
}
