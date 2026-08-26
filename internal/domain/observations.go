package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type ObservationInput struct {
	ID                string
	ReplicateID       string
	DayNo             int
	NormalGerminated  int
	AbnormalSeedlings int
	HardSeeds         int
	DeadSeeds         int
	UngerminatedSeeds int
	RecordedBy        string
}

type ObservationBatchIssue struct {
	ReplicateID string `json:"replicateId,omitempty"`
	Field       string `json:"field"`
	Code        string `json:"code"`
	Message     string `json:"message"`
}

type ObservationBatchInput struct {
	DayNo        int
	RecordedBy   string
	Observations []ObservationInput
}

func (a *Aggregate) RecordObservation(input ObservationInput, at time.Time) (Event, error) {
	value, err := a.prepareObservation(input, at)
	if err != nil {
		return Event{}, err
	}
	return NewEvent(EventObservationRecorded, at, ObservationRecordedData{Observation: value})
}

func (a *Aggregate) RecordObservationBatch(input ObservationBatchInput, at time.Time) (Event, []ObservationBatchIssue, error) {
	issues := a.ValidateObservationBatch(input)
	if len(issues) > 0 {
		return Event{}, issues, nil
	}
	values := make([]Observation, 0, len(input.Observations))
	for _, item := range input.Observations {
		item.DayNo = input.DayNo
		item.RecordedBy = input.RecordedBy
		value, err := a.prepareObservation(item, at)
		if err != nil {
			return Event{}, nil, err
		}
		values = append(values, value)
	}
	event, err := NewEvent(EventObservationBatchRecorded, at, ObservationBatchRecordedData{DayNo: input.DayNo, RecordedBy: strings.TrimSpace(input.RecordedBy), Observations: values})
	return event, nil, err
}

func (a *Aggregate) ValidateObservationBatch(input ObservationBatchInput) []ObservationBatchIssue {
	issues := make([]ObservationBatchIssue, 0)
	if len(input.Observations) == 0 {
		return []ObservationBatchIssue{{Field: "observations", Code: "REQUIRED", Message: "至少提交一个重复组"}}
	}
	seen := map[string]bool{}
	for _, item := range input.Observations {
		id := strings.TrimSpace(item.ReplicateID)
		if id == "" {
			issues = append(issues, ObservationBatchIssue{Field: "replicateId", Code: "REQUIRED", Message: "重复组不能为空"})
			continue
		}
		if seen[id] {
			issues = append(issues, ObservationBatchIssue{ReplicateID: id, Field: "replicateId", Code: "DUPLICATE", Message: "批量载荷中的重复组编号重复"})
			continue
		}
		seen[id] = true
		item.DayNo = input.DayNo
		item.RecordedBy = input.RecordedBy
		if _, err := a.prepareObservation(item, time.Time{}); err != nil {
			var field string
			var code = "INVALID"
			if typed, ok := err.(*DomainError); ok {
				for key := range typed.Fields {
					field = key
					break
				}
				if typed.Code == CodeNotFound {
					code = "NOT_FOUND"
				}
			}
			if field == "" {
				field = "observation"
			}
			issues = append(issues, ObservationBatchIssue{ReplicateID: id, Field: field, Code: code, Message: err.Error()})
		}
	}
	sort.SliceStable(issues, func(i, j int) bool {
		if issues[i].ReplicateID == issues[j].ReplicateID {
			return issues[i].Field < issues[j].Field
		}
		return issues[i].ReplicateID < issues[j].ReplicateID
	})
	return issues
}

func (a *Aggregate) prepareObservation(input ObservationInput, at time.Time) (Observation, error) {
	if a.Assessment.Status != StatusObserving && a.Assessment.Status != StatusReturned {
		return Observation{}, State("observing 或 returned", a.Assessment.Status)
	}
	r, ok := a.Replicates[input.ReplicateID]
	if !ok {
		return Observation{}, &DomainError{Code: CodeNotFound, Message: "重复组不存在", Fields: map[string]string{"replicateId": "不属于本评定"}}
	}
	if r.Status == ReplicateVoid {
		return Observation{}, Invalid("已作废重复组不能录入观测", map[string]string{"replicateId": "重复组已作废"})
	}
	if !containsDay(a.Protocol.ObservationDays, input.DayNo) {
		return Observation{}, Invalid("观察日不在冻结日程内", map[string]string{"dayNo": "不允许的观察日"})
	}
	counts := []int{input.NormalGerminated, input.AbnormalSeedlings, input.HardSeeds, input.DeadSeeds, input.UngerminatedSeeds}
	total := 0
	for _, n := range counts {
		if n < 0 {
			return Observation{}, Invalid("分类计数不能为负数", map[string]string{"counts": "不能为负数"})
		}
		total += n
	}
	if total != r.SownQuantity {
		return Observation{}, Invalid(fmt.Sprintf("分类计数合计 %d，必须等于播种量 %d", total, r.SownQuantity), map[string]string{"counts": "计数不守恒"})
	}
	if strings.TrimSpace(input.RecordedBy) == "" {
		return Observation{}, Invalid("记录人不能为空", map[string]string{"recordedBy": "不能为空"})
	}
	list := a.Observations[r.ID]
	latestByDay := map[int]Observation{}
	maxDay := 0
	for _, old := range list {
		latestByDay[old.DayNo] = old
		if old.DayNo > maxDay {
			maxDay = old.DayNo
		}
	}
	old, revising := latestByDay[input.DayNo]
	if !revising && input.DayNo < maxDay {
		return Observation{}, Invalid("不能在更晚观察日之后补录较早日期", map[string]string{"dayNo": "晚于当前修订位置"})
	}
	if !revising {
		nextIndex := len(latestByDay)
		if nextIndex >= len(a.Protocol.ObservationDays) || a.Protocol.ObservationDays[nextIndex] != input.DayNo {
			return Observation{}, Invalid("必须按冻结日程依次录入观察日", map[string]string{"dayNo": "观察日次序不连续"})
		}
	}
	previousNormal := 0
	days := make([]int, 0, len(latestByDay))
	for day := range latestByDay {
		if day < input.DayNo {
			days = append(days, day)
		}
	}
	sort.Ints(days)
	if len(days) > 0 {
		previousNormal = latestByDay[days[len(days)-1]].NormalGerminated
	}
	if input.NormalGerminated < previousNormal {
		return Observation{}, Invalid("正常发芽累计数不能低于前一观察日", map[string]string{"normalGerminated": "不能低于前一观察日"})
	}
	revision := 1
	if revising {
		revision = old.RevisionNo + 1
	}
	value := Observation{ID: input.ID, ReplicateID: input.ReplicateID, DayNo: input.DayNo, NormalGerminated: input.NormalGerminated, AbnormalSeedlings: input.AbnormalSeedlings, HardSeeds: input.HardSeeds, DeadSeeds: input.DeadSeeds, UngerminatedSeeds: input.UngerminatedSeeds, RevisionNo: revision, RecordedBy: strings.TrimSpace(input.RecordedBy), RecordedAt: at.UTC()}
	return value, nil
}

func containsDay(days []int, target int) bool {
	for _, day := range days {
		if day == target {
			return true
		}
	}
	return false
}

func LatestObservations(list []Observation) map[int]Observation {
	result := map[int]Observation{}
	for _, observation := range list {
		old, ok := result[observation.DayNo]
		if !ok || observation.RevisionNo > old.RevisionNo {
			result[observation.DayNo] = observation
		}
	}
	return result
}

func (a *Aggregate) HasTerminalObservation(replicateID string) bool {
	if a.Protocol == nil {
		return false
	}
	_, ok := LatestObservations(a.Observations[replicateID])[a.Protocol.TerminationDay]
	return ok
}
