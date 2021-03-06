package main

import (
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/massiveco/aws-autoscalinggroup-dns-sd/reactor"
)

var r reactor.Reactor

func init() {
	r = reactor.New(nil)
}

func main() {
	lambda.Start(r.Handle)
}
