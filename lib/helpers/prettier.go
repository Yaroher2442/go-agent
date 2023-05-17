package helpers

import (
	"github.com/jedib0t/go-pretty/v6/progress"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"os"
	"time"
)

var (
	PrettyFlagAutoStop                 = TruePtr()
	PrettyFlagHideETA                  = TruePtr()
	PrettyFlagHideETAOverall           = TruePtr()
	PrettyFlagHideOverallTracker       = TruePtr()
	PrettyFlagHidePercentage           = TruePtr()
	PrettyFlagHideTime                 = TruePtr()
	PrettyFlagHideValue                = TruePtr()
	PrettyFlagShowSpeed                = TruePtr()
	PrettyFlagShowSpeedOverall         = TruePtr()
	PrettyFlagShowPinned         *bool = TruePtr()
)

func ConstructTable(header *table.Row) table.Writer {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.SetOutputMirror(os.Stdout)
	if header != nil {
		t.AppendHeader(*header)
	}
	return t
}

func NewProgressBar(numTasks int, updateFrequency int, trackers ...*progress.Tracker) *progress.Writer {
	pw := progress.NewWriter()
	pw.SetAutoStop(*PrettyFlagAutoStop)
	pw.SetTrackerLength(30)
	pw.SetMessageWidth(24)
	pw.SetNumTrackersExpected(numTasks)
	pw.SetSortBy(progress.SortByPercentDsc)
	pw.SetStyle(progress.StyleDefault)
	pw.SetTrackerPosition(progress.PositionRight)
	pw.SetUpdateFrequency(time.Millisecond * time.Duration(updateFrequency))
	pw.Style().Colors = progress.StyleColorsExample
	pw.Style().Options.PercentFormat = "%4.1f%%"
	pw.Style().Visibility.ETA = *PrettyFlagHideETA
	pw.Style().Visibility.ETAOverall = *PrettyFlagHideETAOverall
	pw.Style().Visibility.Percentage = *PrettyFlagHidePercentage
	pw.Style().Visibility.Speed = *PrettyFlagShowSpeed
	pw.Style().Visibility.SpeedOverall = *PrettyFlagShowSpeedOverall
	pw.Style().Visibility.Time = *PrettyFlagHideTime
	pw.Style().Visibility.TrackerOverall = *PrettyFlagHideOverallTracker
	pw.Style().Visibility.Value = *PrettyFlagHideValue
	pw.Style().Visibility.Pinned = *PrettyFlagShowPinned
	for _, track := range trackers {
		pw.AppendTracker(track)
	}
	return &pw
}

func NewTracker(total int, units *progress.Units, color *text.Color, message string) *progress.Tracker {
	var targetUnit progress.Units
	if units == nil {
		targetUnit = progress.UnitsDefault
	}
	var targetColor text.Color
	if color == nil {
		targetColor = text.FgCyan
	}
	return &progress.Tracker{Message: targetColor.Sprint(message), Total: int64(total), Units: targetUnit}

}
