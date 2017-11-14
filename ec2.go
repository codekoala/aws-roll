package roll

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

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
