package protocol

import (
	"seed-vigor-gate/internal/domain"
	"time"
)

type Engine struct{}

func NewEngine() *Engine { return &Engine{} }

func (e *Engine) PrepareSnapshot(input domain.ProtocolSnapshot, submittedQuantity int) (domain.ProtocolSnapshot, []Issue, error) {
	issues := Validate(input, submittedQuantity)
	if len(issues) > 0 {
		return input, issues, nil
	}
	digest, err := domain.ProtocolDigest(input)
	if err != nil {
		return input, nil, err
	}
	input.ContentDigest = digest
	return input, nil, nil
}

func (e *Engine) Progress(aggregate *domain.Aggregate) ProgressProjection {
	return BuildProgress(aggregate)
}

func (e *Engine) Calculate(aggregate *domain.Aggregate, at time.Time) (domain.Metrics, error) {
	series, err := BuildSeries(aggregate)
	if err != nil {
		return domain.Metrics{}, err
	}
	calc := CalculateValues(*aggregate.Protocol, series)
	hits := Evaluate(*aggregate.Protocol, calc, aggregate)
	return domain.Metrics{FinalGerminationRate: calc.FinalRate, Dispersion: calc.Dispersion, ThresholdDay: calc.ThresholdDay, Decision: Decision(hits), RuleHits: hits, CalculatedAt: at.UTC(), SourceVersion: aggregate.Assessment.Version}, nil
}
