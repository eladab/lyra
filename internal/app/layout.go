package app

type layout struct {
	width, height          int
	editorCols, editorRows int
	previewCols            int
}

func measure(width, height int) layout {
	return measureSplit(width, height, .5)
}

func measureSplit(width, height int, split float64) layout {
	l := layout{width: max(1, width), height: max(1, height)}
	l.editorRows = max(1, height-3) // title, pane headings, footer
	l.editorCols = l.width
	if width >= 88 {
		l.editorCols = max(20, min(width-21, int(float64(width-1)*split)))
		l.previewCols = width - l.editorCols - 1
	}
	return l
}
