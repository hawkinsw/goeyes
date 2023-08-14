package goeyes

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"sync"
	"time"
)

func Smoke() bool {
	return true
}

type HappyResult int

const (
	IPv4 HappyResult = iota
	IPv6
	Invalid
)

type InvalidHostname struct {
	msg string
}

func (ih InvalidHostname) Error() string {
	return ih.msg
}

func makeInvalidHostnameError(bad string, good string) InvalidHostname {
	return InvalidHostname{fmt.Sprintf("malformed host: the parsed host (%v) did not match the given host (%v)", bad, good)}
}

func HappyEyeballs(ctx context.Context, host string, port int) (HappyResult, error) {
	// We will assume that they gave us a hostname. We add an "http://" in front in order
	// to make Parse happy!
	parseResult, err := url.Parse("http://" + host)
	if err != nil {
		return Invalid, err
	}
	if parseResult.Host != host {
		return Invalid, makeInvalidHostnameError(parseResult.Host, host)
	}

	dnsResults, err := net.LookupHost(host)
	if err != nil {
		return Invalid, err
	}

	v4exists := false
	v4Address := net.IPv4zero
	v6exists := false
	v6Address := net.IPv6zero

	// 1. Let's determine if there is a quad AAAA
	for _, result := range dnsResults {
		parsedAddress := net.ParseIP(result)
		if parsedAddress == nil {
			continue
		}
		if parsedAddress.To4() == nil {
			v6exists = true
			v6Address = parsedAddress
		} else {
			v4exists = true
			v4Address = parsedAddress
		}
	}

	// Neither v4 nor v6 was found!
	if !(v4exists || v6exists) {
		err := net.DNSError{}
		err.Err = "Found neither a v4 no a v6 DNS result"
		err.IsNotFound = true
		err.IsTemporary = false
		err.IsTimeout = false
		return Invalid, &err
	}

	// One or the other was found, but not both!
	if v4exists != v6exists {
		if v4exists {
			return IPv4, nil
		}
		return IPv6, nil
	}

	// Here is the hard case! Both were found! Now we have to race!

	dialWg := sync.WaitGroup{}
	dialWg.Add(2)
	dialResultChan := make(chan HappyResult, 2)
	result := Invalid
	var resultErr error = nil

	parentContext := context.Background()
	if ctx != nil {
		parentContext = ctx
	}
	dialerContext, dialerContextCancel := context.WithCancel(parentContext)
	timeout, timeoutCancel := context.WithTimeout(dialerContext, 5*time.Second)
	dialer := net.Dialer{}

	go func() {
		dialWg.Wait()
		// This will result when both dialers are done. If this is the only result
		// that the loop gets, then it will send Invalid back to the caller and everything
		// is good (for being bad!)
		dialResultChan <- Invalid
	}()

	go func() {
		_, connectionError := dialer.DialContext(dialerContext, "tcp", fmt.Sprintf("%v:%v", v4Address.String(), port))
		if connectionError == nil {
			dialResultChan <- IPv4
		}
		dialWg.Done()
	}()

	go func() {
		_, connectionError := dialer.DialContext(dialerContext, "tcp6", fmt.Sprintf("[%v]:%v", v6Address.String(), port))
		if connectionError == nil {
			dialResultChan <- IPv6
		}
		dialWg.Done()
	}()

race:
	for {
		select {
		case result = <-dialResultChan:
			break race
		case <-timeout.Done():
			resultErr = os.ErrDeadlineExceeded
			break race
		}
	}
	timeoutCancel()
	dialerContextCancel()

	if result == Invalid {
		resultErr = os.ErrDeadlineExceeded
	}
	return result, resultErr
}
