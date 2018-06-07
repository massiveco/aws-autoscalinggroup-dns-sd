package reactor

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/massiveco/aws-hostname/identity"
)

// Reactor Manage ASG Lifecycle for individual nodes
type Reactor struct {
	route53Client     *route53.Route53
	autoscalingClient *autoscaling.AutoScaling
	ec2Client         *ec2.EC2
}

type autoscalingEvent struct {
	EC2InstanceID        string `json:"EC2InstanceId"`
	AutoScalingGroupName string
	Event                string
}
type srvRecord struct {
	Name string
}

// New Create a new reactor to ASG SNS Events
func New(sess *session.Session) Reactor {

	if sess == nil {
		sess, _ = session.NewSessionWithOptions(session.Options{
			Config: aws.Config{
				HTTPClient: http.DefaultClient,
			},
			SharedConfigState: session.SharedConfigEnable,
		})
	}

	return Reactor{
		route53Client:     route53.New(sess),
		autoscalingClient: autoscaling.New(sess),
		ec2Client:         ec2.New(sess),
	}
}

func (r Reactor) processEvent(event autoscalingEvent) (*string, error) {
	asg, err := r.lookupAutoScalingGroup(event.AutoScalingGroupName)
	if err != nil {
		return nil, err
	}
	instances, err := r.lookupInstances(event.AutoScalingGroupName)
	if err != nil {
		return nil, err
	}

	zoneID := extractTag("massive:DNS-SD:Route53:zone", asg.Tags)
	zone, err := r.route53Client.GetHostedZone(&route53.GetHostedZoneInput{Id: zoneID})
	if err != nil {
		return nil, err
	}

	srvNames := strings.Split(*extractTag("massive:DNS-SD:names", asg.Tags), ",")
	srvPorts := strings.Split(*extractTag("massive:DNS-SD:ports", asg.Tags), ",")
	var changes []*route53.Change
	serviceDiscoveryRecords := make(map[string][]*route53.ResourceRecord, len(srvNames))

	for _, serviceName := range srvNames {
		serviceDiscoveryRecords[serviceName] = []*route53.ResourceRecord{}
	}

	for _, instance := range instances {
		if *instance.State.Code == 48 {
			continue
		}
		shouldAdd := event.Event == "autoscaling:EC2_INSTANCE_LAUNCH" || event.EC2InstanceID != *instance.InstanceId

		if shouldAdd {
			hostname, _ := identity.GenerateHostname(*instance)
			fqdn := strings.Join([]string{*hostname, *zone.HostedZone.Name}, ".")

			for i, serviceName := range srvNames {
				serviceDiscoveryRecords[serviceName] = append(serviceDiscoveryRecords[serviceName], &route53.ResourceRecord{Value: aws.String(strings.Join([]string{"0 0", srvPorts[i], fqdn}, " "))})
			}
		}
	}

	for name, records := range serviceDiscoveryRecords {

		action := aws.String("UPSERT")
		if len(records) == 0 {
			action = aws.String("DELETE")
			output, err := r.route53Client.ListResourceRecordSets(&route53.ListResourceRecordSetsInput{HostedZoneId: zoneID, StartRecordName: &name})
			if err != nil {
				log.Fatal(err)
			}
			records = output.ResourceRecordSets[0].ResourceRecords

		}
		changes = append(changes, &route53.Change{
			Action: action,
			ResourceRecordSet: &route53.ResourceRecordSet{
				Name:            aws.String(name),
				Type:            aws.String("SRV"),
				ResourceRecords: records,
				TTL:             aws.Int64(60),
			},
		})
	}
	params := &route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: changes,
		},
		HostedZoneId: aws.String(*zoneID),
	}

	_, err = r.route53Client.ChangeResourceRecordSets(params)
	if err != nil {
		return nil, err
	}

	return &event.EC2InstanceID, nil
}

func (r Reactor) lookupInstances(name string) ([]*ec2.Instance, error) {
	var instances []*ec2.Instance
	filter := ec2.Filter{Name: aws.String("tag:aws:autoscaling:groupName"), Values: []*string{aws.String(name)}}
	output, err := r.ec2Client.DescribeInstances(&ec2.DescribeInstancesInput{Filters: []*ec2.Filter{&filter}})
	if err != nil {
		return nil, err
	}
	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return nil, errors.New("No reservations/Instances found")
	}

	for _, res := range output.Reservations {
		for _, i := range res.Instances {
			instances = append(instances, i)
		}
	}
	return instances, nil
}

func (r Reactor) lookupAutoScalingGroup(name string) (*autoscaling.Group, error) {
	output, err := r.autoscalingClient.DescribeAutoScalingGroups(&autoscaling.DescribeAutoScalingGroupsInput{AutoScalingGroupNames: []*string{aws.String(name)}})
	if err != nil {
		return nil, err
	}
	if len(output.AutoScalingGroups) == 0 {
		return nil, errors.New("AutoScalingGroup not found")
	}

	return output.AutoScalingGroups[0], nil
}

//Handle a request
func (r Reactor) Handle(req events.SNSEvent) (*string, error) {
	if len(req.Records) == 0 || req.Records[0].SNS.Message == "" {
		return nil, errors.New("No SNS Message found")
	}

	message := req.Records[0].SNS.Message
	var evt autoscalingEvent
	err := json.Unmarshal([]byte(message), &evt)
	if err != nil {
		return nil, err
	}
	return r.processEvent(evt)
}

func extractTag(tagName string, tags []*autoscaling.TagDescription) *string {

	for _, tag := range tags {
		if *tag.Key == tagName {
			return tag.Value
		}
	}

	return nil
}

func extractTagFromInstance(tagName string, tags []*ec2.Tag) *string {

	for _, tag := range tags {
		if *tag.Key == tagName {
			return tag.Value
		}
	}

	return nil
}
