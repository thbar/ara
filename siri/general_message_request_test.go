package siri

import (
	"testing"
	"time"
)

func Test_SIRIGeneralMessageRequest(t *testing.T) {
	expectedXML := `<sw:GetGeneralMessage xmlns:sw="http://wsdl.siri.org.uk" xmlns:siri="http://www.siri.org.uk/siri" xmlns:sws="http://wsdl.siri.org.uk/siri">
	<ServiceRequestInfo>
		<siri:RequestTimestamp>2016-09-21T20:14:46.000Z</siri:RequestTimestamp>
		<siri:RequestorRef>ref</siri:RequestorRef>
		<siri:MessageIdentifier>MessageId</siri:MessageIdentifier>
	</ServiceRequestInfo>
	<Request version="2.0:FR-IDF-2.4">
		<siri:RequestTimestamp>2016-09-21T20:14:46.000Z</siri:RequestTimestamp>
		<siri:MessageIdentifier>MessageId</siri:MessageIdentifier>
	</Request>
</sw:GetGeneralMessage>`

	requestTimestamp := time.Date(2016, time.September, 21, 20, 14, 46, 0, time.UTC)

	gmRequest := &SIRIGetGeneralMessageRequest{
		RequestorRef: "ref",
	}
	gmRequest.RequestTimestamp = requestTimestamp
	gmRequest.MessageIdentifier = "MessageId"

	xml, err := gmRequest.BuildXML()
	if err != nil {
		t.Fatal(err)
	}
	if expectedXML != xml {
		t.Errorf("Wrong XML for Request:\n got:\n%v\nwant:\n%v", xml, expectedXML)
	}

}
