package ui

const (
	minPanelWidth        = 10
	minPanelHeight       = 5
	horizontalOverhead   = 6
	verticalOverhead     = 2
	timelineFixedHeight  = 7
	propertiesFixedWidth = 30
)

type PanelDimensions struct {
	PreviewWidth            int
	PreviewHeight           int
	PropertiesWidth         int
	PropertiesHeight        int
	TimelineWidth           int
	TimelineHeight          int
	PreviewContentWidth     int
	PreviewContentHeight    int
	PropertiesContentWidth  int
	PropertiesContentHeight int
	TimelineContentWidth    int
	TimelineContentHeight   int
}

func CalculatePanelDimensions(termWidth, termHeight int) PanelDimensions {
	timelineHeight := timelineFixedHeight
	topRowHeight := termHeight - timelineHeight

	propertiesWidth := propertiesFixedWidth
	previewWidth := termWidth - propertiesWidth

	return PanelDimensions{
		PreviewWidth:            previewWidth,
		PreviewHeight:           topRowHeight,
		PropertiesWidth:         propertiesWidth,
		PropertiesHeight:        topRowHeight,
		TimelineWidth:           termWidth,
		TimelineHeight:          timelineHeight,
		PreviewContentWidth:     max(0, previewWidth-horizontalOverhead),
		PreviewContentHeight:    max(0, topRowHeight-verticalOverhead),
		PropertiesContentWidth:  max(0, propertiesWidth-horizontalOverhead),
		PropertiesContentHeight: max(0, topRowHeight-verticalOverhead),
		TimelineContentWidth:    max(0, termWidth-horizontalOverhead),
		TimelineContentHeight:   max(0, timelineHeight-verticalOverhead),
	}
}
