package main

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

// App holds all references for App components of the App.App as well as all
// info regarding an App instance
type App struct {
	app              *tview.Application
	list             *tview.List
	input            *tview.InputField
	statusBox        *tview.TextView
	widebox          *WideBox
	wideboxFakeFocus bool
	inputText        string
	spells           *[]Spell
	dataChan         chan []Spell
	statusChan       chan string
}

// Instantiate a new app ready to run
func newApp() *App {
	ui := App{}

	app := tview.NewApplication().EnableMouse(false)
	ui.app = app

	// instantiate all parts of the UI
	ui.list = getList()
	ui.input = getInputField(ui.setInputText)
	ui.statusBox = getStatusBox()
	ui.widebox = getWideBox()
	ui.dataChan = make(chan []Spell)
	ui.statusChan = make(chan string)
	go ui.waitForData()
	go ui.waitForStatuses()
	go loadAllData(ui.dataChan, ui.statusChan)

	// set the global input handler
	ui.app.SetInputCapture(ui.handleInput)

	// bind all the UI elements to the app instance
	root := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(tview.NewFlex().
			AddItem(ui.list, 0, 3, true).
			AddItem(ui.widebox.grid, 0, 7, false), 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(ui.input, 0, 3, false).
			AddItem(ui.statusBox, 0, 7, false), 1, 0, false)

	app.SetRoot(root, true)

	app.SetFocus(ui.input)
	ui.focusList()

	return &ui
}

// Run the app
func (app *App) Run() {
	app.app.Run()
}

// Waits for the data in the data channel. When the data arrives, it halts
func (app *App) waitForData() {
	for v := range app.dataChan {
		app.spells = &v
		app.setSpells()
		// for some reason, the screen isn't auto updated on the initial spell set
		// so it has to be so manually.
		// TODO: find a fix without invoking this function below - low priority
		app.app.Draw()
	}
}

// Waits for the statuses in the status channel. Stays always open
func (app *App) waitForStatuses() {
	for v := range app.statusChan {
		app.setStatus(v)
	}
}

// The main app global input handler
func (app *App) handleInput(event *tcell.EventKey) *tcell.EventKey {
	// if status exists, clear it
	if app.getStatus() != "" {
		app.setStatus("")
	}
	switch event.Key() {
	case tcell.KeyEnter:
		//ui.wideboxFakeFocus = true
		if spell := app.currentSelectedSpell(); spell != nil {
			app.widebox.SetSpell(spell)
		}
	case tcell.KeyUp, tcell.KeyCtrlK:
		if app.wideboxFakeFocus {
			app.widebox.ScrollUp()
			break
		}
		app.list.SetCurrentItem(app.list.GetCurrentItem() - 1)
	case tcell.KeyCtrlJ, tcell.KeyDown:
		if app.wideboxFakeFocus {
			app.widebox.ScrollDown()
			break
		}
		item := app.list.GetCurrentItem()
		if item >= app.list.GetItemCount()-1 {
			app.list.SetCurrentItem(0)
			break
		}
		app.list.SetCurrentItem(item + 1)
	// tcell.KeyCtrlBackspace doesn't exist for whatever reason
	case tcell.KeyCtrlD:
		app.input.SetText("")
	case tcell.KeyLeft, tcell.KeyCtrlH:
		app.focusList()
	case tcell.KeyRight, tcell.KeyCtrlL:
		app.focusWideBox()
	case tcell.KeyTab:
		app.switchFocus()
	}
	return event
}

func (app *App) setStatus(text string) {
	app.statusBox.SetText(text)
}

func (app *App) getStatus() string {
	// the bool signifies whether should the color tags be stripped off or not
	return app.statusBox.GetText(true)
}

// Switched focus between the list on the left and the main content area to the right
func (app *App) switchFocus() {
	if app.wideboxFakeFocus {
		app.focusList()
		return
	}
	app.focusWideBox()
}

// Focuses the list
func (app *App) focusList() {
	app.wideboxFakeFocus = false
	app.list.SetBorderAttributes(tcell.AttrBold)
	app.widebox.grid.SetBorderAttributes(tcell.AttrNone)
}

// Focuses the main content area on the right
func (app *App) focusWideBox() {
	app.wideboxFakeFocus = true
	app.list.SetBorderAttributes(tcell.AttrNone)
	app.widebox.grid.SetBorderAttributes(tcell.AttrBold)
}

// Filters and sets the spells from app.spells and updates it on the screen
// Does NOT update app.spells
func (app *App) setSpells() {
	app.list.Clear()
	for i, s := range *app.spells {
		lname := strings.ToLower(s.Name)
		linput := strings.ToLower(app.inputText)

		if strings.Contains(lname, linput) {
			nameString := strconv.Itoa(s.Level) + " " + s.Name

			if s.Ritual || s.Concentration {
				_, _, w, _ := app.list.Box.GetInnerRect()
				padLen := w - len(nameString)
				padNum := 0
				if s.Concentration {
					padNum++
				}
				if s.Ritual {
					padNum++
				}

				if padLen >= 3 {
					nameString += strings.Repeat(" ", padLen-padNum)
					if s.Concentration {
						nameString += "C"
					}
					if s.Ritual {
						nameString += "R"
					}
				}
			}

			app.list.AddItem(highlight(nameString, app.inputText), strconv.Itoa(i), 0, nil)
		}
	}
}

// Returns the current selected spell. Returns nil if there are no spells in the list
func (app App) currentSelectedSpell() *Spell {
	if app.list.GetItemCount() < 1 {
		return nil
	}
	_, s := app.list.GetItemText(app.list.GetCurrentItem())
	index, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	spells := *app.spells
	return &spells[index]
}

// Handler than should be ran on every text input change. Filters the spell list
// on text update.
func (app *App) setInputText(text string) {
	// focus the list on key input if the main content box happens to be focused atm
	app.focusList()
	app.inputText = text
	app.setSpells()
}

// Highlight a substring in a string regardless of it's capitalisation.
// May not work properly with unicode
func highlight(str, substr string) string {
	lname := strings.ToLower(str)
	linput := strings.ToLower(substr)
	parts := strings.Split(lname, linput)
	pre := "[#ff0000]"
	post := "[white]"

	// precalculated lengths for small performance benefits
	prelen := len(pre)
	postlen := len(post)
	patternlen := len(substr)

	var final string
	for i, w := range parts {
		startx := len(final)
		if i > 1 {
			startx -= (i - 1) * (prelen + postlen)
		}
		if i != 0 {
			final += pre + str[startx:startx+patternlen] + post
			startx += patternlen
		}
		final += str[startx : startx+len(w)]
	}
	return final
}

// Returns a pointer to a new list element preconfigured for the app
func getList() *tview.List {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true)
	return list
}

// Returns a pointer to a new input element preconfigured for the app
func getInputField(processInput func(string)) *tview.InputField {
	input := tview.NewInputField().
		SetLabel(">>> ").
		SetFieldBackgroundColor(tview.Styles.PrimitiveBackgroundColor)
	input.SetChangedFunc(processInput)
	return input
}

func getStatusBox() *tview.TextView {
	box := tview.NewTextView().SetTextAlign(tview.AlignRight)
	box.SetBorder(false)
	return box
}
