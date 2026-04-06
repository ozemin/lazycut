package ui

const (
	minPanelWidth       = 10
	minPanelHeight      = 5
	horizontalOverhead  = 6
	verticalOverhead    = 2
	timelineFixedHeight = 7
	propertiesLineHeight = 1
)

type PanelDimensions struct {
	PreviewWidth         int
	PreviewHeight        int
	PropertiesLineWidth  int
	TimelineWidth        int
	TimelineHeight       int
	PreviewContentWidth  int
	PreviewContentHeight int
	TimelineContentWidth  int
	TimelineContentHeight int
}

func CalculatePanelDimensions(termWidth, termHeight int) PanelDimensions {
	timelineHeight := timelineFixedHeight
	topRowHeight := termHeight - timelineHeight - propertiesLineHeight

	return PanelDimensions{
		PreviewWidth:          termWidth,
		PreviewHeight:         topRowHeight,
		PropertiesLineWidth:   termWidth,
		TimelineWidth:         termWidth,
		TimelineHeight:        timelineHeight,
		PreviewContentWidth:   max(0, termWidth-horizontalOverhead),
		PreviewContentHeight:  max(0, topRowHeight-verticalOverhead),
		TimelineContentWidth:  max(0, termWidth-horizontalOverhead),
		TimelineContentHeight: max(0, timelineHeight-verticalOverhead),
	}
}
