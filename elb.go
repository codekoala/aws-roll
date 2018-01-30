package roll

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elb"
	"go.uber.org/zap"
)

type ELBInstanceFunc func(string, string) bool

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
		Log.Warn("error describing load balancers", zap.Error(err))
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
	log := Log.With(zap.String("elb", loadBalancer))

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
	log := Log.With(zap.String("elb", loadBalancer))

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

// GetInstanceState gets the current state of the specified instance for the specified load balancer
func (this *ELB) GetInstanceState(instanceId, loadBalancer string) (state string) {
	state = "Gone"
	log := Log.With(zap.String("elb", loadBalancer))

	input := &elb.DescribeInstanceHealthInput{
		LoadBalancerName: aws.String(loadBalancer),
		Instances: []*elb.Instance{{
			InstanceId: aws.String(instanceId),
		}},
	}

	log.Info("checking instance state")
	out, err := this.svc.DescribeInstanceHealth(input)
	if err != nil {
		log.Warn("error checking instance state", zap.Error(err))
		return
	}

	for _, st := range out.InstanceStates {
		if *st.InstanceId != instanceId {
			continue
		}

		state = *st.State
		break
	}

	return
}

// IsHealthy checks to see if the specified instance is in service for the specified load balancer
func (this *ELB) IsHealthy(instanceId, loadBalancer string) bool {
	return this.HasState(instanceId, loadBalancer, "InService")
}

// IsOutOfService checks to see if the specified instance is out of service for the specified load balancer
func (this *ELB) IsOutOfService(instanceId, loadBalancer string) bool {
	return this.HasState(instanceId, loadBalancer, "OutOfService", "Gone")
}

// HasState checks to see if the specified instance has the specified state for the specified load balancer
func (this *ELB) HasState(instanceId, loadBalancer string, states ...string) bool {
	state := this.GetInstanceState(instanceId, loadBalancer)
	for _, expected := range states {
		if state == expected {
			return true
		}
	}

	return false
}
