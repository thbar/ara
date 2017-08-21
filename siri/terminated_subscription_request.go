package siri

import (
	"github.com/jbowtie/gokogiri"
	"github.com/jbowtie/gokogiri/xml"
)

type XMLTerminatedSubscriptionRequest struct {
	RequestXMLStructure

	subscriptionRef string
}

const TerminateSubscriptionRequest = `<ns1:TerminateSubscriptionRequest xmlns:ns1="http://wsdl.siri.org.uk">
  <ServiceRequestInfo
   xmlns:ns2="http://www.ifopt.org.uk/acsb"
   xmlns:ns3="http://www.ifopt.org.uk/ifopt"
   xmlns:ns4="http://datex2.eu/schema/2_0RC1/2_0"
   xmlns:ns5="http://www.siri.org.uk/siri"
   xmlns:ns6="http://wsdl.siri.org.uk/siri">
    <ns5:ResponseTimestamp>{{ .ResponseTimestamp.Format "2006-01-02T15:04:05.000Z07:00" }}</ns5:ResponseTimestamp>
    <ns5:RequestorRef>{{.ResponderRef}}</ns5:RequestorRef>
    <ns2:MessageIdentifier>{{.MessageIdentifier}}</ns2:MessageIdentifier>
  </ServiceRequestInfo>
  <Request version="2.0:FR-IDF-2.4">
    <ns5:SubscriptionRef>{{.SubscriptionRef}}</ns5:SubscriptionRef>
  </Request>
  <RequestExtension/>
</ns1:TerminateSubscriptionRequest>`

func NewXMLTerminatedSubscriptionRequest(node xml.Node) *XMLTerminatedSubscriptionRequest {
	xmlTerminatedSubscriptionRequest := &XMLTerminatedSubscriptionRequest{}
	xmlTerminatedSubscriptionRequest.node = NewXMLNode(node)
	return xmlTerminatedSubscriptionRequest
}

func NewXMLTerminatedSubscriptionRequestFromContent(content []byte) (*XMLTerminatedSubscriptionRequest, error) {
	doc, err := gokogiri.ParseXml(content)
	if err != nil {
		return nil, err
	}
	request := NewXMLTerminatedSubscriptionRequest(doc.Root().XmlNode)
	return request, nil
}

func (request *XMLTerminatedSubscriptionRequest) SubscriptionRef() string {
	if request.subscriptionRef == "" {
		request.subscriptionRef = request.findStringChildContent("SubscriptionRef")
	}
	return request.subscriptionRef
}
