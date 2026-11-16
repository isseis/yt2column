// Package devbackend is the development Backend of the article generation
// module: the fake LLM clients of a devfake scenario and its key check
// result. It calls no network, and it does not import the Claude adapter.
package devbackend

import (
	"context"
	"strconv"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/extjob"
	"github.com/isseis/yt2column/internal/llm/devfake"
)

// backend is the development Backend of one scenario.
type backend struct {
	scenario devfake.Scenario
}

// New returns the Backend that serves scenario. NewClients ignores the
// configuration it is given and returns new fake clients for each run, so
// runs share no call counts.
func New(scenario devfake.Scenario) extjob.Backend {
	return backend{scenario: scenario}
}

func (b backend) NewClients(config.LLMConfig) (extjob.Deps, error) {
	job := b.scenario.NewJob()
	return extjob.Deps{Generation: job.Generation(), Review: job.Review()}, nil
}

// CheckKey returns the scenario's key check result without any call.
func (b backend) CheckKey(context.Context, config.LLMConfig) error {
	return b.scenario.CheckKeyResult()
}

// Info declares the fake implementation and the scenario's values.
func (b backend) Info() extjob.BackendInfo {
	s := b.scenario.Spelling()
	// ParseScenario accepted the delay as whole decimal seconds up to
	// MaxDelaySeconds, so the conversion cannot fail; a failure reports 0.
	delay, _ := strconv.Atoi(s.DelaySeconds)
	return extjob.FakeInfo(extjob.FakeScenario{
		Article:       s.Article,
		Failure:       s.Failure,
		FailAt:        s.FailAt,
		DelaySeconds:  delay,
		SettingsCheck: s.SettingsCheck,
	})
}
