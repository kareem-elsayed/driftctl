package aws

import (
	"strings"

	"github.com/cloudskiff/driftctl/pkg/alerter"
	"github.com/cloudskiff/driftctl/pkg/parallel"

	awsdeserializer "github.com/cloudskiff/driftctl/pkg/resource/aws/deserializer"

	"github.com/cloudskiff/driftctl/pkg/remote/deserializer"

	"github.com/cloudskiff/driftctl/pkg/resource"
	resourceaws "github.com/cloudskiff/driftctl/pkg/resource/aws"
	"github.com/cloudskiff/driftctl/pkg/terraform"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/zclconf/go-cty/cty"
)

type Route53RecordSupplier struct {
	reader       terraform.ResourceReader
	deserializer deserializer.CTYDeserializer
	client       route53iface.Route53API
	runner       *terraform.ParallelResourceReader
	alerter      *alerter.Alerter
}

func NewRoute53RecordSupplier(runner *parallel.ParallelRunner, client route53iface.Route53API, alerter *alerter.Alerter) *Route53RecordSupplier {
	return &Route53RecordSupplier{
		terraform.Provider(terraform.AWS),
		awsdeserializer.NewRoute53RecordDeserializer(),
		client,
		terraform.NewParallelResourceReader(runner),
		alerter,
	}
}

func (s Route53RecordSupplier) Resources() ([]resource.Resource, error) {

	zones, err := s.listZones()
	if err != nil {
		handled := handleListAwsErrorWithMessage(err, resourceaws.AwsRoute53RecordResourceType, s.alerter, resourceaws.AwsRoute53ZoneResourceType)
		if !handled {
			return nil, err
		}
		return []resource.Resource{}, nil
	}

	for _, zone := range zones {
		if err := s.listRecordsForZone(zone[0], zone[1]); err != nil {
			handled := handleListAwsError(err, resourceaws.AwsRoute53RecordResourceType, s.alerter)
			if !handled {
				return nil, err
			}
		}
	}

	results, err := s.runner.Wait()
	if err != nil {
		return nil, err
	}
	return s.deserializer.Deserialize(results)
}

func (s Route53RecordSupplier) listZones() ([][2]string, error) {
	results := make([][2]string, 0)
	zones, err := listAwsRoute53Zones(s.client)
	if err != nil {
		return nil, err
	}

	for _, hostedZone := range zones {
		results = append(results, [2]string{strings.TrimPrefix(*hostedZone.Id, "/hostedzone/"), *hostedZone.Name})
	}

	return results, nil
}

func (s Route53RecordSupplier) listRecordsForZone(zoneId string, zoneName string) error {

	records, err := listAwsRoute53Records(s.client, zoneId)

	if err != nil {
		return err
	}

	for _, raw := range records {
		rawType := *raw.Type
		rawName := *raw.Name
		rawSetIdentifier := raw.SetIdentifier
		s.runner.Run(func() (cty.Value, error) {
			vars := []string{
				zoneId,
				strings.ToLower(strings.TrimSuffix(rawName, ".")),
				rawType,
			}
			if rawSetIdentifier != nil {
				vars = append(vars, *rawSetIdentifier)
			}

			record, err := s.reader.ReadResource(
				terraform.ReadResourceArgs{
					Ty: resourceaws.AwsRoute53RecordResourceType,
					ID: strings.Join(vars, "_"),
				},
			)
			if err != nil {
				return cty.NilVal, err
			}

			return *record, nil
		})

	}
	return nil
}

func listAwsRoute53Records(client route53iface.Route53API, zoneId string) ([]*route53.ResourceRecordSet, error) {
	var results []*route53.ResourceRecordSet
	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneId),
	}
	err := client.ListResourceRecordSetsPages(input, func(res *route53.ListResourceRecordSetsOutput, lastPage bool) bool {
		results = append(results, res.ResourceRecordSets...)
		return !lastPage
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}
