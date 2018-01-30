package roll

import (
	"os"
	"os/exec"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"go.uber.org/zap"
)

var Log *zap.Logger

func init() {
	Log, _ = zap.NewProduction()
}

func Roll(command string) (err error) {
	var lane string

	meta := getMetadata()
	instanceId := meta.InstanceID
	sess := session.Must(session.NewSession(&aws.Config{
		Region:      aws.String(meta.Region),
		Credentials: getAWSCredentials(),
	}))

	ec2s := NewEC2(sess)
	if lane, err = ec2s.GetInstanceLane(instanceId); err != nil {
		return
	}

	Log = Log.With(
		zap.String("instanceId", instanceId),
		zap.String("region", meta.Region),
		zap.String("lane", lane),
	)

	Log.Info("found instance")
	elbs := NewELB(sess)
	loadBalancers := elbs.FindForInstance(instanceId)
	if len(loadBalancers) == 0 {
		return ErrInstanceNotAssigned
	}

	Log.Info("found instance load balancers", zap.Any("elbs", loadBalancers))
	withAllELBs(elbs.DeregisterInstance, instanceId, loadBalancers...)

	Log.Info("waiting for instance to be out of service")
	for {
		if withAllELBs(elbs.IsOutOfService, instanceId, loadBalancers...) {
			break
		}

		time.Sleep(time.Second * 5)
	}

	Log.Info("running command", zap.Any("command", command))
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Log.Debug("command", zap.Any("cmd", cmd))
	if err := cmd.Run(); err != nil {
		Log.Warn("failed to run command", zap.Error(err))
	}

	withAllELBs(elbs.RegisterInstance, instanceId, loadBalancers...)

	Log.Info("waiting for instance to be healthy")
	for {
		if withAllELBs(elbs.IsHealthy, instanceId, loadBalancers...) {
			break
		}

		time.Sleep(time.Second * 5)
	}

	Log.Info("done")

	return nil
}
