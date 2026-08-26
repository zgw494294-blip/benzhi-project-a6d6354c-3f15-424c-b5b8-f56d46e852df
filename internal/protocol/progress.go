package protocol

import (
	"math"
	"seed-vigor-gate/internal/domain"
	"sort"
)

type ProgressSource struct {
	ReplicateID    string  `json:"replicateId"`
	ReplicateLabel string  `json:"replicateLabel"`
	RevisionNo     int     `json:"revisionNo"`
	Rate           float64 `json:"rate"`
}

type OutlierHint struct {
	ReplicateID    string  `json:"replicateId"`
	ReplicateLabel string  `json:"replicateLabel"`
	RevisionNo     int     `json:"revisionNo"`
	Rate           float64 `json:"rate"`
	MedianRate     float64 `json:"medianRate"`
	Difference     float64 `json:"difference"`
	Message        string  `json:"message"`
}

type DailyProgress struct {
	DayNo               int              `json:"dayNo"`
	CoveredGroups       int              `json:"coveredGroups"`
	EffectiveGroups     int              `json:"effectiveGroups"`
	AverageNormalRate   *float64         `json:"averageNormalRate,omitempty"`
	Increment           *float64         `json:"increment,omitempty"`
	GroupRange          *float64         `json:"groupRange,omitempty"`
	MissingReplicateIDs []string         `json:"missingReplicateIds"`
	Sources             []ProgressSource `json:"sources"`
	Outliers            []OutlierHint    `json:"outliers"`
}

type ProgressProjection struct {
	Days     []DailyProgress `json:"days"`
	Advisory string          `json:"advisory"`
}

func BuildProgress(aggregate *domain.Aggregate) ProgressProjection {
	projection := ProgressProjection{Days: []DailyProgress{}, Advisory: "离群提示仅供复核定位，不改变既有资格判定语义。"}
	if aggregate.Protocol == nil {
		return projection
	}
	effective := make([]domain.Replicate, 0)
	for _, replicate := range aggregate.SortedReplicates() {
		if replicate.Status != domain.ReplicateVoid {
			effective = append(effective, replicate)
		}
	}
	var previous *float64
	for _, day := range aggregate.Protocol.ObservationDays {
		point := DailyProgress{DayNo: day, EffectiveGroups: len(effective), MissingReplicateIDs: []string{}, Sources: []ProgressSource{}, Outliers: []OutlierHint{}}
		rates := make([]float64, 0, len(effective))
		for _, replicate := range effective {
			observation, ok := domain.LatestObservations(aggregate.Observations[replicate.ID])[day]
			if !ok {
				point.MissingReplicateIDs = append(point.MissingReplicateIDs, replicate.ID)
				continue
			}
			rate := round2(percentage(observation.NormalGerminated, replicate.SownQuantity))
			rates = append(rates, rate)
			point.Sources = append(point.Sources, ProgressSource{ReplicateID: replicate.ID, ReplicateLabel: replicate.Label, RevisionNo: observation.RevisionNo, Rate: rate})
		}
		point.CoveredGroups = len(rates)
		if len(rates) > 0 {
			total, minimum, maximum := 0.0, rates[0], rates[0]
			for _, rate := range rates {
				total += rate
				if rate < minimum {
					minimum = rate
				}
				if rate > maximum {
					maximum = rate
				}
			}
			average := round2(total / float64(len(rates)))
			rangeValue := round2(maximum - minimum)
			point.AverageNormalRate = &average
			point.GroupRange = &rangeValue
			if previous != nil {
				increment := round2(average - *previous)
				point.Increment = &increment
			}
			previous = &average
			point.Outliers = outlierHints(point.Sources, rates)
		} else {
			previous = nil
		}
		projection.Days = append(projection.Days, point)
	}
	return projection
}

func outlierHints(sources []ProgressSource, rates []float64) []OutlierHint {
	if len(rates) < 3 {
		return []OutlierHint{}
	}
	medianRate := median(rates)
	deviations := make([]float64, len(rates))
	for index, rate := range rates {
		deviations[index] = math.Abs(rate - medianRate)
	}
	threshold := math.Max(10, 2*median(deviations))
	hints := make([]OutlierHint, 0)
	for _, source := range sources {
		difference := round2(math.Abs(source.Rate - medianRate))
		if difference > threshold {
			hints = append(hints, OutlierHint{ReplicateID: source.ReplicateID, ReplicateLabel: source.ReplicateLabel, RevisionNo: source.RevisionNo, Rate: source.Rate, MedianRate: round2(medianRate), Difference: difference, Message: "该组明显偏离当日组间中位水平，请复核录入或试验条件"})
		}
	}
	return hints
}

func median(values []float64) float64 {
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 == 1 {
		return ordered[middle]
	}
	return (ordered[middle-1] + ordered[middle]) / 2
}
