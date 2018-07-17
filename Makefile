export AWS_REGION=us-east-1
export GOCACHE=off

test:
	go test ./...

build:
	GOOS=linux go build .
	zip deploy.zip aws-autoscalinggroup-dns-sd
	rm aws-autoscalinggroup-dns-sd

release: build
	aws s3 cp ./deploy.zip s3://ma.ssive.co/lambdas/massive_autoscaling_dns_sd.zip
