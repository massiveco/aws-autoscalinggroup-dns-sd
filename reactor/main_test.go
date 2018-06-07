package reactor

import (
	"io/ioutil"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestHandleWithNoSNSMessage(t *testing.T) {
	r := New(nil)

	_, err := r.Handle(events.SNSEvent{})
	if err == nil {
		t.Error("Expected error: No SNS Message found")
	}
}

func TestHandle(t *testing.T) {
	r := New(nil)

	fixtureBytes, _ := ioutil.ReadFile("./fixtures/sns_entity.json")
	sns := events.SNSEntity{Message: string(fixtureBytes[:])}

	_, err := r.Handle(events.SNSEvent{Records: []events.SNSEventRecord{{SNS: sns}}})
	if err != nil {
		t.Error(err)
	}
	t.Error(true)
}
