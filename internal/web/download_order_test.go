package web

import (
	"reflect"
	"testing"
)

func TestDownloadEpisodeOrderStartsWithUnseen(t *testing.T) {
	got := downloadEpisodeOrder([]int{1,2,3,4,5,6,7,8,9,10,11,12}, 5)
	want := []int{6,7,8,9,10,11,12,1,2,3,4,5}
	if !reflect.DeepEqual(got, want) { t.Fatalf("order=%v want=%v", got, want) }
}

func TestDownloadEpisodeOrderKeepsEpisodeZeroAfterUnseen(t *testing.T) {
	got := downloadEpisodeOrder([]int{0,1,2,3}, 1)
	want := []int{2,3,0,1}
	if !reflect.DeepEqual(got, want) { t.Fatalf("order=%v want=%v", got, want) }
}

func TestDownloadEpisodeOrderWhenNothingSeen(t *testing.T) {
	got := downloadEpisodeOrder([]int{1,2,3}, 0)
	want := []int{1,2,3}
	if !reflect.DeepEqual(got, want) { t.Fatalf("order=%v want=%v", got, want) }
}
