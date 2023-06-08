package leakybucket

import (
	"fmt"
	"time"

	"github.com/crowdsecurity/crowdsec/pkg/types"
	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
)

func TimeMachinePour(l *Leaky, msg types.Event) {
	var (
		d   time.Time
		err error
	)
	if msg.MarshaledTime == "" {
		log.WithFields(log.Fields{
			"evt_type": msg.Line.Labels["type"],
			"evt_src":  msg.Line.Src,
			"scenario": l.Name,
		}).Warningf("Trying to process event without evt.StrTime. Event cannot be poured to scenario")
		return
	}
	err = d.UnmarshalText([]byte(msg.MarshaledTime))
	fmt.Printf("Before\ntimestamp: %s\nmsg: %+v\nLimiter last: %s\n", d.String(), spew.Sdump(msg.Line.Raw), l.Limiter.Dump().Last.String())
	if err != nil {
		log.Warningf("Failed unmarshaling event time (%s) : %v", msg.MarshaledTime, err)
		return
	}

	l.Total_count += 1
	l.mutex.Lock()
	if l.First_ts.IsZero() {
		l.logger.Debugf("First event, bucket creation time : %s", d)
		l.First_ts = d
	}
	l.Last_ts = d
	l.mutex.Unlock()

	//fmt.Printf("bucket before: %+v\n", *l)
	if l.Limiter.AllowN(d, 1) {
		l.logger.Tracef("Time-Pouring event %s (tokens:%f)", d, l.Limiter.GetTokensCount())
		l.Queue.Add(msg)
	} else {
		l.Ovflw_ts = d
		l.logger.Debugf("Bucket overflow at %s", l.Ovflw_ts)
		l.Queue.Add(msg)
		l.Out <- l.Queue
	}
	fmt.Printf("After\ntimestamp: %s\nmsg: %+v\nLimiter last: %s\n", d.String(), spew.Sdump(msg.Line.Raw), l.Limiter.Dump().Last.String())
	//fmt.Printf("bucket after: %+v\n", *l)
}

func NewTimeMachine(g BucketFactory) *Leaky {
	l := NewLeaky(g)
	g.logger.Tracef("Instantiating timeMachine bucket")
	l.Pour = TimeMachinePour
	l.Mode = types.TIMEMACHINE
	return l
}
