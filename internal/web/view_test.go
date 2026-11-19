package web

import (
	"testing"
	"time"

	"benevole-ape/internal/model"
)

func TestSlotStatusWords(t *testing.T) {
	for _, c := range []struct {
		sv   model.SlotView
		want string
	}{
		{model.SlotView{Slot: model.Slot{MinPeople: 3}, Missing: 3}, "Il reste 3 places"},
		{model.SlotView{Slot: model.Slot{MinPeople: 2}, Count: 1, Missing: 1}, "Il reste 1 place"},
		{model.SlotView{Slot: model.Slot{MinPeople: 3, MaxPeople: 4}, Count: 3}, "Assez de monde, encore 1 place"},
		{model.SlotView{Slot: model.Slot{MinPeople: 2}, Count: 2}, "Assez de monde, vous pouvez venir"},
		{model.SlotView{Slot: model.Slot{MinPeople: 2, MaxPeople: 2}, Count: 2, Full: true}, "Complet"},
	} {
		if got := slotStatus(&c.sv); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.sv, got, c.want)
		}
	}
}

func TestUrgentSlots(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2027, 6, 19, h, 0, 0, 0, time.UTC) }
	slot := func(id int64, h, missing int, can bool) *model.SlotView {
		return &model.SlotView{Slot: model.Slot{ID: id, StartsAt: at(h)}, Missing: missing, CanSignUp: can}
	}
	p := &model.Planning{Stands: []*model.StandView{
		{Slots: []*model.SlotView{slot(1, 14, 1, true), slot(2, 16, 3, true), slot(3, 15, 3, true)}},
		{Slots: []*model.SlotView{slot(4, 9, 5, false), slot(5, 10, 0, true), slot(6, 11, 2, true)}},
	}}
	var ids []int64
	for _, s := range urgentSlots(p, 3) {
		ids = append(ids, s.ID)
	}
	// Most missing first, then the earliest; never a slot one cannot take.
	if want := []int64{3, 2, 6}; len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Errorf("got %v, want %v", ids, want)
	}
}
