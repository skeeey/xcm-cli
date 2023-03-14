package recorder

import (
	"context"

	"github.com/openshift/library-go/pkg/operator/events"
)

type DumbRecorder struct{}

var _ events.Recorder = &DumbRecorder{}

func (*DumbRecorder) Event(reason, message string)                                     {}
func (*DumbRecorder) Eventf(reason, messageFmt string, args ...interface{})            {}
func (*DumbRecorder) Warning(reason, message string)                                   {}
func (*DumbRecorder) Warningf(reason, messageFmt string, args ...interface{})          {}
func (d *DumbRecorder) ForComponent(componentName string) events.Recorder              { return d }
func (d *DumbRecorder) WithComponentSuffix(componentNameSuffix string) events.Recorder { return d }
func (d *DumbRecorder) WithContext(ctx context.Context) events.Recorder                { return d }
func (*DumbRecorder) ComponentName() string                                            { return "dumb" }
func (*DumbRecorder) Shutdown()                                                        {}
