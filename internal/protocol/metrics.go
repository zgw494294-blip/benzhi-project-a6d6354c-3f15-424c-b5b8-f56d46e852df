package protocol

import (
	"fmt"
	"math"
	"seed-vigor-gate/internal/domain"
	"sort"
)

type ReplicateSeries struct {
	Replicate domain.Replicate
	Latest    map[int]domain.Observation
}

type Calculation struct {
	FinalRate            float64
	Dispersion           float64
	ThresholdDay         int
	ActiveReplicateCount int
}

func BuildSeries(aggregate *domain.Aggregate) ([]ReplicateSeries, error) {
	if aggregate.Protocol == nil {
		return nil, domain.Invalid("缺少冻结方案", nil)
	}
	series := make([]ReplicateSeries, 0)
	for _, replicate := range aggregate.SortedReplicates() {
		if replicate.Status == domain.ReplicateVoid {
			continue
		}
		latest := domain.LatestObservations(aggregate.Observations[replicate.ID])
		if _, ok := latest[aggregate.Protocol.TerminationDay]; !ok {
			return nil, domain.Invalid("有效重复组缺少终止日观测", map[string]string{"replicateId": replicate.ID})
		}
		series = append(series, ReplicateSeries{Replicate: replicate, Latest: latest})
	}
	if len(series) == 0 {
		return nil, domain.Invalid("没有可用于计算的有效重复组", nil)
	}
	return series, nil
}

func CalculateValues(snapshot domain.ProtocolSnapshot, series []ReplicateSeries) Calculation {
	rates := make([]float64, 0, len(series))
	for _, item := range series {
		observation := item.Latest[snapshot.TerminationDay]
		rates = append(rates, percentage(observation.NormalGerminated, item.Replicate.SownQuantity))
	}
	minRate, maxRate, total := rates[0], rates[0], 0.0
	for _, rate := range rates {
		total += rate
		if rate < minRate {
			minRate = rate
		}
		if rate > maxRate {
			maxRate = rate
		}
	}
	thresholdDay := 0
	for _, day := range snapshot.ObservationDays {
		dayTotal, available := 0.0, true
		for _, item := range series {
			observation, ok := item.Latest[day]
			if !ok {
				available = false
				break
			}
			dayTotal += percentage(observation.NormalGerminated, item.Replicate.SownQuantity)
		}
		if available && dayTotal/float64(len(series)) >= snapshot.MinimumGerminationRate {
			thresholdDay = day
			break
		}
	}
	return Calculation{FinalRate: round2(total / float64(len(rates))), Dispersion: round2(maxRate - minRate), ThresholdDay: thresholdDay, ActiveReplicateCount: len(series)}
}

func percentage(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) * 100 / float64(denominator)
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }

func DescribeSeries(snapshot domain.ProtocolSnapshot, series []ReplicateSeries) []string {
	result := make([]string, 0, len(series))
	for _, item := range series {
		observation := item.Latest[snapshot.TerminationDay]
		result = append(result, fmt.Sprintf("%s=%.2f%%", item.Replicate.Label, percentage(observation.NormalGerminated, item.Replicate.SownQuantity)))
	}
	sort.Strings(result)
	return result
}
