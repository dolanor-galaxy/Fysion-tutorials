package main

import (
	"errors"
	"image/color"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"fysion.app/internal/dialogs"
	"fysion.app/internal/editors"
)

type gui struct {
	win   fyne.Window
	title binding.String

	fileTree binding.URITree
	content  *container.DocTabs
	openTabs map[fyne.URI]*container.TabItem
}

func (g *gui) makeBanner() fyne.CanvasObject {
	title := canvas.NewText("App Creator", theme.ForegroundColor())
	title.TextSize = 14
	title.TextStyle = fyne.TextStyle{Bold: true}

	g.title.AddListener(binding.NewDataListener(func() {
		name, _ := g.title.Get()
		if name == "" {
			name = "App Creator"
		}
		title.Text = name
		title.Refresh()
	}))

	home := widget.NewButtonWithIcon("", theme.HomeIcon(), func() {})
	left := container.NewHBox(home, title)

	logo := canvas.NewImageFromResource(resourceLogoPng)
	logo.FillMode = canvas.ImageFillContain

	return container.NewStack(container.NewPadded(left), container.NewPadded(logo))
}

func (g *gui) makeGUI() fyne.CanvasObject {
	top := g.makeBanner()
	g.fileTree = binding.NewURITree()
	files := widget.NewTreeWithData(g.fileTree, func(branch bool) fyne.CanvasObject {
		return widget.NewLabel("filename.jpg")
	}, func(data binding.DataItem, branch bool, obj fyne.CanvasObject) {
		l := obj.(*widget.Label)
		u, _ := data.(binding.URI).Get()

		name := u.Name()
		l.SetText(name)
	})
	files.OnSelected = func(id widget.TreeNodeID) {
		u, err := g.fileTree.GetValue(id)
		if err != nil {
			dialog.ShowError(err, g.win)
			return
		}

		g.openFile(u)
	}

	left := widget.NewAccordion(
		widget.NewAccordionItem("Files", files),
		widget.NewAccordionItem("Screens", widget.NewLabel("TODO screens")),
	)
	left.Open(0)
	left.MultiOpen = true

	right := widget.NewRichTextFromMarkdown("## Settings")

	name, _ := g.title.Get()
	window := container.NewInnerWindow(name,
		widget.NewLabel("App Preview Here"),
	)
	window.CloseIntercept = func() {}

	picker := widget.NewSelect([]string{"Desktop", "iPhone 15 Max"}, func(string) {})
	picker.Selected = "Desktop"

	preview := container.NewBorder(container.NewHBox(picker), nil, nil, nil, container.NewCenter(window))
	content := container.NewStack(canvas.NewRectangle(color.Gray{Y: 0xee}),
		container.NewPadded(preview))

	g.content = container.NewDocTabs(
		container.NewTabItem("Preview", content),
	)
	g.content.CloseIntercept = func(item *container.TabItem) {
		var u fyne.URI
		for child, childItem := range g.openTabs {
			if childItem == item {
				u = child
			}
		}

		if u != nil {
			delete(g.openTabs, u)
		}
		g.content.Remove(item)
	}

	dividers := [3]fyne.CanvasObject{
		widget.NewSeparator(), widget.NewSeparator(), widget.NewSeparator(),
	}
	objs := []fyne.CanvasObject{g.content, top, left, right, dividers[0], dividers[1], dividers[2]}
	return container.New(newFysionLayout(top, left, right, g.content, dividers), objs...)
}

func (g *gui) makeMenu() *fyne.MainMenu {
	file := fyne.NewMenu("File",
		fyne.NewMenuItem("Open Project", g.openProjectDialog),
	)

	return fyne.NewMainMenu(file)
}

func (g *gui) openFile(u fyne.URI) {
	listable, err := storage.CanList(u)
	if listable || err != nil {
		// TODO should we unselect this item
		return
	}

	if item, ok := g.openTabs[u]; ok {
		g.content.Select(item)
		return
	}

	edit := editors.ForURI(u)

	item := container.NewTabItem(u.Name(), edit)
	if g.openTabs == nil {
		g.openTabs = make(map[fyne.URI]*container.TabItem)
	}
	g.openTabs[u] = item

	g.content.Append(item)
	g.content.Select(item)
}

func (g *gui) openProjectDialog() {
	dialog.ShowFolderOpen(func(dir fyne.ListableURI, err error) {
		if err != nil {
			dialog.ShowError(err, g.win)
			return
		}
		if dir == nil {
			return
		}

		g.openProject(dir)
	}, g.win)
}

func (g *gui) showCreate(w fyne.Window) {
	var wizard *dialogs.Wizard
	intro := widget.NewLabel(`Here you can create new project!

Or open an existing one that you created earlier.`)

	open := widget.NewButton("Open Project", func() {
		wizard.Hide()
		g.openProjectDialog()
	})
	create := widget.NewButton("Create Project", func() {
		wizard.Push("Project Details", g.makeCreateDetail(wizard))
	})
	create.Importance = widget.HighImportance

	buttons := container.NewGridWithColumns(2, open, create)
	home := container.NewVBox(intro, buttons)

	wizard = dialogs.NewWizard("Create Project", home)
	wizard.Show(w)
	wizard.Resize(home.MinSize().AddWidthHeight(40, 80)) //fyne.NewSize(360, 200))
}

func (g *gui) makeCreateDetail(wizard *dialogs.Wizard) fyne.CanvasObject {
	homeDir, _ := os.UserHomeDir()
	parent := storage.NewFileURI(homeDir)
	chosen, _ := storage.ListerForURI(parent)

	name := widget.NewEntry()
	name.Validator = func(in string) error {
		if in == "" {
			return errors.New("project name is required")
		}

		return nil
	}
	var dir *widget.Button
	dir = widget.NewButton(chosen.Name(), func() {
		d := dialog.NewFolderOpen(func(l fyne.ListableURI, err error) {
			if err != nil || l == nil {
				return
			}

			chosen = l
			dir.SetText(l.Name())
		}, g.win)

		d.SetLocation(chosen)
		d.Show()
	})

	form := widget.NewForm(
		widget.NewFormItem("Name", name),
		widget.NewFormItem("Parent Directory", dir),
	)
	form.OnSubmit = func() {
		project, err := createProject(name.Text, chosen)
		if err != nil {
			dialog.ShowError(err, g.win)
			return
		}
		wizard.Hide()
		g.openProject(project)
	}

	return form
}
