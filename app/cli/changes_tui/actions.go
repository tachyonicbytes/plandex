package changes_tui

import (
	"fmt"
	"log"
	"plandex/api"
	"plandex/lib"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/fatih/color"
	"github.com/muesli/reflow/wrap"
	"github.com/plandex/plandex/shared"
)

func (m *changesUIModel) rejectChange() {
	if m.selectionInfo == nil || m.selectionInfo.currentRep == nil {
		log.Println("can't drop change; no change is currently selected")
		return
	}

	err := api.Client.RejectReplacement(lib.CurrentPlanId, lib.CurrentBranch, m.selectionInfo.currentRes.Id, m.selectionInfo.currentRep.Id)

	if err != nil {
		log.Printf("error dropping change: %v", err)
		return
	}

}

func (m changesUIModel) copyCurrentChange() error {
	selectionInfo := m.selectionInfo
	if selectionInfo.currentRep == nil {
		return fmt.Errorf("no change is currently selected")
	}

	// Copy the 'New' content of the replacement to the clipboard
	if err := clipboard.WriteAll(selectionInfo.currentRep.New); err != nil {
		return fmt.Errorf("failed to copy to clipboard: %v", err)
	}

	return nil
}

func (m *changesUIModel) left() {
	if m.selectedFileIndex > 0 {
		m.selectedFileIndex--
		m.selectedReplacementIndex = 0
	}
	m.setSelectionInfo()
	m.updateMainView(true)
}

func (m *changesUIModel) right() {
	paths := m.currentPlan.PlanResult.SortedPaths

	if m.selectedFileIndex < len(paths)-1 {
		m.selectedFileIndex++
		m.selectedReplacementIndex = 0
	}

	m.setSelectionInfo()
	m.updateMainView(true)
}

func (m *changesUIModel) up() {
	if m.selectedReplacementIndex > 0 {
		// log.Println("up")
		m.selectedReplacementIndex--
		m.setSelectionInfo()
		m.updateMainView(true)
	}
}

func (m *changesUIModel) down() {
	var currentReplacements []*shared.Replacement
	if m.selectionInfo != nil {
		currentReplacements = m.selectionInfo.currentReplacements
	}

	max := len(currentReplacements) - 1

	// allow for selection of 'full file' option at bottom of replacement list in sidebar
	if m.currentPlan.PlanResult.NumPendingForPath(m.selectionInfo.currentPath) > 0 {
		max++
	}

	// allow for selection of 'new file' option at top of replacement list in sidebar
	if m.hasNewFile() {
		max++
	}

	if m.selectedReplacementIndex < max {
		// log.Println("down")
		m.selectedReplacementIndex++
		m.setSelectionInfo()
		m.updateMainView(true)
	}
}

func (m *changesUIModel) scrollUp() {
	if m.selectionInfo.currentRep == nil && m.fileScrollable() {
		m.fileViewport.LineUp(1)
	} else if m.selectedViewport == 0 && m.oldScrollable() {
		m.changeOldViewport.LineUp(1)
	} else if m.newScrollable() {
		m.changeNewViewport.LineUp(1)
	}
}

func (m *changesUIModel) scrollDown() {
	if m.selectionInfo.currentRep == nil && m.fileScrollable() {
		m.fileViewport.LineDown(1)
	} else if m.selectedViewport == 0 && m.oldScrollable() {
		m.changeOldViewport.LineDown(1)
	} else if m.newScrollable() {
		m.changeNewViewport.LineDown(1)
	}
}

func (m *changesUIModel) pageUp() {
	if m.selectionInfo.currentRep == nil && m.fileScrollable() {
		m.fileViewport.ViewUp()
	} else if m.selectedViewport == 0 && m.oldScrollable() {
		m.changeOldViewport.ViewUp()
	} else if m.newScrollable() {
		m.changeNewViewport.ViewUp()
	}
}

func (m *changesUIModel) pageDown() {
	if m.selectionInfo.currentRep == nil && m.fileScrollable() {
		m.fileViewport.ViewDown()
	} else if m.selectedViewport == 0 && m.oldScrollable() {
		m.changeOldViewport.ViewDown()
	} else if m.newScrollable() {
		m.changeNewViewport.ViewDown()
	}
}

func (m *changesUIModel) switchView() {
	m.selectedViewport = 1 - m.selectedViewport
	m.updateMainView(false)
}

func (m *changesUIModel) windowResized(w, h int) {
	m.width = w
	m.height = h
	didInit := false
	if !m.ready {
		m.initViewports()
		m.ready = true
		didInit = true
	}
	m.updateMainView(didInit)
}

func (m *changesUIModel) updateMainView(scrollReplacement bool) {
	// log.Println("updateMainView")

	// var updateMsg types.ChangesUIViewportsUpdate

	if m.selectedNewFile() || m.selectedFullFile() {
		context := m.currentPlan.ContextsByPath[m.selectionInfo.currentPath]
		var originalFile string
		if context != nil {
			originalFile = context.Body
		}

		var updatedFile string

		if m.selectedNewFile() {
			updatedFile = m.selectionInfo.currentRes.Content
		} else {
			updatedFile = m.currentPlan.CurrentPlanFiles.Files[m.selectionInfo.currentPath]
		}

		wrapWidth := m.fileViewport.Width - 2
		fileSegments := []string{}
		replacementSegments := map[int]bool{}

		if context == nil {
			// the file is new, so all lines are new and should be highlighted
			fileSegments = append(fileSegments, updatedFile)
			replacementSegments[0] = true
		} else {
			lastFoundIdx := 0
			updatedLines := strings.Split(updatedFile, "\n")
			for i, line := range updatedLines {
				fileSegments = append(fileSegments, line+"\n")
				originalIdx := strings.Index(originalFile, line)
				if originalIdx == -1 || originalIdx < lastFoundIdx {
					replacementSegments[i] = true
				} else {
					lastFoundIdx = originalIdx + len(line)
					replacementSegments[i] = false
				}
			}
		}

		for i, segment := range fileSegments {
			wrapped := wrap.String(segment, wrapWidth)
			isReplacement := replacementSegments[i]
			if isReplacement {
				lines := strings.Split(wrapped, "\n")
				for j, line := range lines {
					lines[j] = color.New(color.FgHiGreen).Sprint(line)
				}
				wrapped = strings.Join(lines, "\n")
			}
			fileSegments[i] = wrapped
		}

		m.fileViewport.SetContent(strings.Join(fileSegments, ""))

	} else {
		oldRes := m.getReplacementOldDisplay()
		m.changeOldViewport.SetContent(oldRes.oldDisplay)
		// log.Println("set old content")
		newContent, newContentDisplay := m.getReplacementNewDisplay(oldRes.prependContent, oldRes.appendContent)
		m.changeNewViewport.SetContent(newContentDisplay)
		// log.Println("set new content")

		if scrollReplacement {
			m.scrollReplacementIntoView(oldRes.old, newContent, oldRes.numLinesPrepended)
		}
	}

	m.updateViewportSizes()
}