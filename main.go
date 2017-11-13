package main

import (
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/mitchellh/go-homedir"
	"go.uber.org/zap"
)

var (
	AWS_SHARED_CREDENTIALS_FILE = os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	AWS_PROFILE                 = os.Getenv("AWS_PROFILE")

	DefaultSharedCredentialsFile = "~/.aws/config"
	DefaultProfile               = "default"

	log *zap.Logger
)

type ELBInstanceFunc func(string, string) bool

func init() {
	if AWS_SHARED_CREDENTIALS_FILE == "" {
		AWS_SHARED_CREDENTIALS_FILE = DefaultSharedCredentialsFile
	}

	if AWS_PROFILE == "" {
		AWS_PROFILE = DefaultProfile
	}
}

func main() {
	log, _ = zap.NewProduction()
	meta := getMetadata()
	instanceId := meta.InstanceID

	cmdArgs := os.Args[1:]
	if len(cmdArgs) == 0 {
		log.Warn("no command specified; aborting")
		os.Exit(1)
	}

	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(meta.Region),
		Credentials: getAWSCredentials(),
	}))

	ec2s := NewEC2(sess)
	elbs := NewELB(sess)

	lane, err := ec2s.GetInstanceLane(instanceId)
	if err != nil {
		panic(err)
	}

	log = log.With(
		zap.String("instanceId", instanceId),
		zap.String("region", meta.Region),
		zap.String("lane", lane),
	)

	log.Info("found instance")
	loadBalancers := elbs.FindForInstance(instanceId)
	if len(loadBalancers) == 0 {
		log.Warn("instance is assigned to no load balancers; aborting")
		os.Exit(2)
	}

	log.Info("found instance load balancers", zap.Any("elbs", loadBalancers))
	withAllELBs(elbs.DeregisterInstance, instanceId, loadBalancers...)

	log.Info("running command", zap.Any("command", cmdArgs))
	cmd := exec.Command("bash", "-c", cmdArgs[0])
	log.Debug("command", zap.Any("cmd", cmd))
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Warn("failed to run command", zap.String("output", string(out)), zap.Error(err))
	}

	withAllELBs(elbs.RegisterInstance, instanceId, loadBalancers...)

	log.Info("waiting for instance to be healthy")
	for {
		if withAllELBs(elbs.IsHealthy, instanceId, loadBalancers...) {
			break
		}

		time.Sleep(time.Second * 5)
	}

	log.Info("done")
}

// getMetadata returns information about the current EC2 instance
func getMetadata() (data ec2metadata.EC2InstanceIdentityDocument) {
	var err error

	log.Info("retrieving instance metadata")
	svc := ec2metadata.New(session.New(&aws.Config{}))
	if data, err = svc.GetInstanceIdentityDocument(); err != nil {
		log.Warn("failed to retrieve instance metadata", zap.Error(err))
		return
	}

	//log.Info("instance metadata", zap.Any("data", data))

	return data
}

// getAWSCredentials loads the credentials we need to access the AWS API.
func getAWSCredentials() (creds *credentials.Credentials) {
	creds = credentials.NewEnvCredentials()
	if _, err := creds.Get(); err != nil {
		path, _ := homedir.Expand(AWS_SHARED_CREDENTIALS_FILE)
		creds = credentials.NewSharedCredentials(path, AWS_PROFILE)
	}

	return creds
}

type EC2 struct {
	svc *ec2.EC2
}

func NewEC2(sess *session.Session) *EC2 {
	return &EC2{
		svc: ec2.New(sess),
	}
}

// GetInstanceLane returns the name of the lane in which the current instance resides
func (this *EC2) GetInstanceLane(instanceId string) (lane string, err error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{{
			Name:   aws.String("instance-id"),
			Values: []*string{&instanceId},
		}},
	}

	out, err := this.svc.DescribeInstances(input)
	if err != nil {
		return "broken", err
	}

	for _, rez := range out.Reservations {
		for _, inst := range rez.Instances {
			for _, tag := range inst.Tags {
				switch *tag.Key {
				case "Lane":
					lane = *tag.Value
					return lane, nil
				}
			}
		}
	}

	return "not-set", nil
}

type ELB struct {
	svc *elb.ELB
}

func NewELB(sess *session.Session) *ELB {
	return &ELB{
		svc: elb.New(sess),
	}
}

func withAllELBs(do ELBInstanceFunc, instanceId string, loadBalancers ...string) (result bool) {
	for idx, lb := range loadBalancers {
		res := do(instanceId, lb)
		if idx == 0 {
			result = res
		} else {
			result = result && res
		}
	}

	return result
}

// FindForInstance returns a slice of all ELBs to which the specified instance belongs
func (this *ELB) FindForInstance(instanceId string) (lbs []string) {
	input := &elb.DescribeLoadBalancersInput{}

	out, err := this.svc.DescribeLoadBalancers(input)
	if err != nil {
		log.Warn("error describing load balancers", zap.Error(err))
		return
	}

	for _, lbd := range out.LoadBalancerDescriptions {
		for _, inst := range lbd.Instances {
			if *inst.InstanceId == instanceId {
				lbs = append(lbs, *lbd.LoadBalancerName)
			}
		}
	}

	return
}

// DeregisterInstance attempts to remove the specified instance from the specified load balancer
func (this *ELB) DeregisterInstance(instanceId, loadBalancer string) (deregistered bool) {
	log := log.With(zap.String("elb", loadBalancer))

	input := &elb.DeregisterInstancesFromLoadBalancerInput{
		LoadBalancerName: aws.String(loadBalancer),
		Instances: []*elb.Instance{{
			InstanceId: aws.String(instanceId),
		}},
	}

	log.Info("deregistering instance from elb")
	out, err := this.svc.DeregisterInstancesFromLoadBalancer(input)
	if err != nil {
		log.Warn("error deregistering instance", zap.Error(err))
		return
	}
	log.Info("load balancer instances", zap.Any("instances", out.Instances))

	deregistered = true
	for _, inst := range out.Instances {
		if *inst.InstanceId == instanceId {
			deregistered = false
		}
	}

	return deregistered
}

// RegisterInstance attempts to register the specified instance with the specified load balancer
func (this *ELB) RegisterInstance(instanceId, loadBalancer string) (registered bool) {
	log := log.With(zap.String("elb", loadBalancer))

	input := &elb.RegisterInstancesWithLoadBalancerInput{
		LoadBalancerName: aws.String(loadBalancer),
		Instances: []*elb.Instance{{
			InstanceId: aws.String(instanceId),
		}},
	}

	log.Info("registering instance with elb")
	out, err := this.svc.RegisterInstancesWithLoadBalancer(input)
	if err != nil {
		log.Warn("error registering instance", zap.Error(err))
		return
	}
	log.Info("load balancer instances", zap.Any("instances", out.Instances))

	for _, inst := range out.Instances {
		if *inst.InstanceId == instanceId {
			registered = true
		}
	}

	return registered
}

// IsHealthy checks to see if the specified instances is in service for the specified load balancer
func (this *ELB) IsHealthy(instanceId, loadBalancer string) (healthy bool) {
	log := log.With(zap.String("elb", loadBalancer))

	input := &elb.DescribeInstanceHealthInput{
		LoadBalancerName: aws.String(loadBalancer),
		Instances: []*elb.Instance{{
			InstanceId: aws.String(instanceId),
		}},
	}

	log.Info("checking instance health")
	out, err := this.svc.DescribeInstanceHealth(input)
	if err != nil {
		log.Warn("error checking instance health", zap.Error(err))
		return
	}

	for _, state := range out.InstanceStates {
		if *state.InstanceId != instanceId {
			continue
		}

		healthy = *state.State == "InService"
	}

	return healthy
}
