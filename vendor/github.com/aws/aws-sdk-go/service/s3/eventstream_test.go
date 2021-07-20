// Code generated by private/model/cli/gen-api/main.go. DO NOT EDIT.

// +build go1.15

package s3

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/corehandlers"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/awstesting/unit"
	"github.com/aws/aws-sdk-go/private/protocol"
	"github.com/aws/aws-sdk-go/private/protocol/eventstream"
	"github.com/aws/aws-sdk-go/private/protocol/eventstream/eventstreamapi"
	"github.com/aws/aws-sdk-go/private/protocol/eventstream/eventstreamtest"
	"github.com/aws/aws-sdk-go/private/protocol/restxml"
)

var _ time.Time
var _ awserr.Error
var _ context.Context
var _ sync.WaitGroup
var _ strings.Reader

func TestSelectObjectContent_Read(t *testing.T) {
	expectEvents, eventMsgs := mockSelectObjectContentReadEvents()
	sess, cleanupFn, err := eventstreamtest.SetupEventStreamSession(t,
		eventstreamtest.ServeEventStream{
			T:      t,
			Events: eventMsgs,
		},
		true,
	)
	if err != nil {
		t.Fatalf("expect no error, %v", err)
	}
	defer cleanupFn()

	svc := New(sess)
	resp, err := svc.SelectObjectContent(nil)
	if err != nil {
		t.Fatalf("expect no error got, %v", err)
	}
	defer resp.GetStream().Close()

	var i int
	for event := range resp.GetStream().Events() {
		if event == nil {
			t.Errorf("%d, expect event, got nil", i)
		}
		if e, a := expectEvents[i], event; !reflect.DeepEqual(e, a) {
			t.Errorf("%d, expect %T %v, got %T %v", i, e, e, a, a)
		}
		i++
	}

	if err := resp.GetStream().Err(); err != nil {
		t.Errorf("expect no error, %v", err)
	}
}

func TestSelectObjectContent_ReadClose(t *testing.T) {
	_, eventMsgs := mockSelectObjectContentReadEvents()
	sess, cleanupFn, err := eventstreamtest.SetupEventStreamSession(t,
		eventstreamtest.ServeEventStream{
			T:      t,
			Events: eventMsgs,
		},
		true,
	)
	if err != nil {
		t.Fatalf("expect no error, %v", err)
	}
	defer cleanupFn()

	svc := New(sess)
	resp, err := svc.SelectObjectContent(nil)
	if err != nil {
		t.Fatalf("expect no error got, %v", err)
	}

	// Assert calling Err before close does not close the stream.
	resp.GetStream().Err()
	select {
	case _, ok := <-resp.GetStream().Events():
		if !ok {
			t.Fatalf("expect stream not to be closed, but was")
		}
	default:
	}

	resp.GetStream().Close()
	<-resp.GetStream().Events()

	if err := resp.GetStream().Err(); err != nil {
		t.Errorf("expect no error, %v", err)
	}
}

func TestSelectObjectContent_ReadUnknownEvent(t *testing.T) {
	expectEvents, eventMsgs := mockSelectObjectContentReadEvents()
	var eventOffset int

	unknownEvent := eventstream.Message{
		Headers: eventstream.Headers{
			eventstreamtest.EventMessageTypeHeader,
			{
				Name:  eventstreamapi.EventTypeHeader,
				Value: eventstream.StringValue("UnknownEventName"),
			},
		},
		Payload: []byte("some unknown event"),
	}

	eventMsgs = append(eventMsgs[:eventOffset],
		append([]eventstream.Message{unknownEvent}, eventMsgs[eventOffset:]...)...)

	expectEvents = append(expectEvents[:eventOffset],
		append([]SelectObjectContentEventStreamEvent{
			&SelectObjectContentEventStreamUnknownEvent{
				Type:    "UnknownEventName",
				Message: unknownEvent,
			},
		},
			expectEvents[eventOffset:]...)...)

	sess, cleanupFn, err := eventstreamtest.SetupEventStreamSession(t,
		eventstreamtest.ServeEventStream{
			T:      t,
			Events: eventMsgs,
		},
		true,
	)
	if err != nil {
		t.Fatalf("expect no error, %v", err)
	}
	defer cleanupFn()

	svc := New(sess)
	resp, err := svc.SelectObjectContent(nil)
	if err != nil {
		t.Fatalf("expect no error got, %v", err)
	}
	defer resp.GetStream().Close()

	var i int
	for event := range resp.GetStream().Events() {
		if event == nil {
			t.Errorf("%d, expect event, got nil", i)
		}
		if e, a := expectEvents[i], event; !reflect.DeepEqual(e, a) {
			t.Errorf("%d, expect %T %v, got %T %v", i, e, e, a, a)
		}
		i++
	}

	if err := resp.GetStream().Err(); err != nil {
		t.Errorf("expect no error, %v", err)
	}
}

func BenchmarkSelectObjectContent_Read(b *testing.B) {
	_, eventMsgs := mockSelectObjectContentReadEvents()
	var buf bytes.Buffer
	encoder := eventstream.NewEncoder(&buf)
	for _, msg := range eventMsgs {
		if err := encoder.Encode(msg); err != nil {
			b.Fatalf("failed to encode message, %v", err)
		}
	}
	stream := &loopReader{source: bytes.NewReader(buf.Bytes())}

	sess := unit.Session
	svc := New(sess, &aws.Config{
		Endpoint:               aws.String("https://example.com"),
		DisableParamValidation: aws.Bool(true),
	})
	svc.Handlers.Send.Swap(corehandlers.SendHandler.Name,
		request.NamedHandler{Name: "mockSend",
			Fn: func(r *request.Request) {
				r.HTTPResponse = &http.Response{
					Status:     "200 OK",
					StatusCode: 200,
					Header:     http.Header{},
					Body:       ioutil.NopCloser(stream),
				}
			},
		},
	)

	resp, err := svc.SelectObjectContent(nil)
	if err != nil {
		b.Fatalf("failed to create request, %v", err)
	}
	defer resp.GetStream().Close()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err = resp.GetStream().Err(); err != nil {
			b.Fatalf("expect no error, got %v", err)
		}
		event := <-resp.GetStream().Events()
		if event == nil {
			b.Fatalf("expect event, got nil, %v, %d", resp.GetStream().Err(), i)
		}
	}
}

func mockSelectObjectContentReadEvents() (
	[]SelectObjectContentEventStreamEvent,
	[]eventstream.Message,
) {
	expectEvents := []SelectObjectContentEventStreamEvent{
		&ContinuationEvent{},
		&EndEvent{},
		&ProgressEvent{
			Details: &Progress{
				BytesProcessed: aws.Int64(1234),
				BytesReturned:  aws.Int64(1234),
				BytesScanned:   aws.Int64(1234),
			},
		},
		&RecordsEvent{
			Payload: []byte("blob value goes here"),
		},
		&StatsEvent{
			Details: &Stats{
				BytesProcessed: aws.Int64(1234),
				BytesReturned:  aws.Int64(1234),
				BytesScanned:   aws.Int64(1234),
			},
		},
	}

	var marshalers request.HandlerList
	marshalers.PushBackNamed(restxml.BuildHandler)
	payloadMarshaler := protocol.HandlerPayloadMarshal{
		Marshalers: marshalers,
	}
	_ = payloadMarshaler

	eventMsgs := []eventstream.Message{
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("Cont"),
				},
			},
		},
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("End"),
				},
			},
		},
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("Progress"),
				},
			},
			Payload: eventstreamtest.MarshalEventPayload(payloadMarshaler, expectEvents[2]),
		},
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("Records"),
				},
			},
			Payload: expectEvents[3].(*RecordsEvent).Payload,
		},
		{
			Headers: eventstream.Headers{
				eventstreamtest.EventMessageTypeHeader,
				{
					Name:  eventstreamapi.EventTypeHeader,
					Value: eventstream.StringValue("Stats"),
				},
			},
			Payload: eventstreamtest.MarshalEventPayload(payloadMarshaler, expectEvents[4]),
		},
	}

	return expectEvents, eventMsgs
}

type loopReader struct {
	source *bytes.Reader
}

func (c *loopReader) Read(p []byte) (int, error) {
	if c.source.Len() == 0 {
		c.source.Seek(0, 0)
	}

	return c.source.Read(p)
}
