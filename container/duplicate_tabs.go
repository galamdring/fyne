package container

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var _ fyne.Widget = (*DuplicateTabs)(nil)

type DuplicateTabs struct {
	widget.BaseWidget

	Items        []*TabItem
	OnChanged    func(*TabItem)
	OnSelected   func(*TabItem)
	OnUnselected func(*TabItem)
	OnAdded      func() *TabItem
	OnRemoved    func(*TabItem)

	current         int
	location        TabLocation
	isTransitioning bool

	popupMenu *widget.PopUpMenu
}

func NewDuplicateTabs(items ...*TabItem) *DuplicateTabs {
	tabs := &DuplicateTabs{}
	tabs.BaseWidget.ExtendBaseWidget(tabs)
	tabs.setItems(items)
	return tabs
}

func (t *DuplicateTabs) onUnselected() func(*TabItem) {
	return t.OnUnselected
}

func (t *DuplicateTabs) onSelected() func(*TabItem) {
	return func(tab *TabItem) {
		if f := t.OnChanged; f != nil {
			f(tab)
		}
		if f := t.OnSelected; f != nil {
			f(tab)
		}
	}
}

func (t *DuplicateTabs) items() []*TabItem {
	return t.Items
}

func (t *DuplicateTabs) setItems(items []*TabItem) {
	t.Items = items
}

func (t *DuplicateTabs) selected() int {
	return t.current
}

func (t *DuplicateTabs) setSelected(i int) {
	t.current = i
}

func (t *DuplicateTabs) tabLocation() TabLocation {
	return t.location
}

func (t *DuplicateTabs) transitioning() bool {
	return t.isTransitioning
}

func (t *DuplicateTabs) setTransitioning(b bool) {
	t.isTransitioning = b
}

// CreateRenderer is a private method to Fyne which links this widget to its renderer
//
// Implements: fyne.Widget
func (t *DuplicateTabs) CreateRenderer() fyne.WidgetRenderer {
	t.BaseWidget.ExtendBaseWidget(t)
	r := &duplicateTabsRenderer{
		baseTabsRenderer: baseTabsRenderer{
			bar:         &fyne.Container{},
			divider:     canvas.NewRectangle(theme.ShadowColor()),
			indicator:   canvas.NewRectangle(theme.PrimaryColor()),
			buttonCache: make(map[*TabItem]*tabButton),
		},
		duplicateTabs: t,
	}
	r.action = r.buildOverflowTabsButton()

	// Initially setup the tab bar to only show one tab, all others will be in overflow.
	// When the widget is laid out, and we know the size, the tab bar will be updated to show as many as can fit.
	r.updateTabs(1)
	r.updateIndicator(false)
	r.applyTheme(t)
	return r
}

// MinSize returns the size that this widget should not sink below
//
// Implements: fyne.CanvasObject
func (d *DuplicateTabs) MinSize() fyne.Size {
	d.BaseWidget.ExtendBaseWidget(d)
	return d.BaseWidget.MinSize()
}

// SelectIndex sets the TabItem at the specific index to be selected and its content visible.
func (t *DuplicateTabs) SelectIndex(index int) {
	selectIndex(t, index)
	t.Refresh()
}

// Select sets the specified TabItem to be selected and its content visible.
func (t *DuplicateTabs) Select(item *TabItem) {
	selectItem(t, item)
	t.Refresh()
}

func (t *DuplicateTabs) Remove(item *TabItem) {
	removeItem(t, item)
	t.Refresh()
}

// Declare conformity with WidgetRenderer interface.
var _ fyne.WidgetRenderer = (*duplicateTabsRenderer)(nil)

type duplicateTabsRenderer struct {
	baseTabsRenderer
	duplicateTabs *DuplicateTabs
}

func (r *duplicateTabsRenderer) Layout(size fyne.Size) {
	// Try render as many tabs as will fit, others will appear in the overflow
	for i := len(r.duplicateTabs.Items); i > 0; i-- {
		r.updateTabs(i)
		barMin := r.bar.MinSize()
		if r.duplicateTabs.location == TabLocationLeading || r.duplicateTabs.location == TabLocationTrailing {
			if barMin.Height <= size.Height {
				// Tab bar is short enough to fit
				break
			}
		} else {
			if barMin.Width <= size.Width {
				// Tab bar is thin enough to fit
				break
			}
		}
	}

	r.layout(r.duplicateTabs, size)
	r.updateIndicator(r.duplicateTabs.transitioning())
	if r.duplicateTabs.transitioning() {
		r.duplicateTabs.setTransitioning(false)
	}
}

func (r *duplicateTabsRenderer) MinSize() fyne.Size {
	return r.minSize(r.duplicateTabs)
}

func (r *duplicateTabsRenderer) Objects() []fyne.CanvasObject {
	return r.objects(r.duplicateTabs)
}

func (r *duplicateTabsRenderer) Refresh() {
	r.Layout(r.duplicateTabs.Size())

	r.refresh(r.duplicateTabs)

	canvas.Refresh(r.duplicateTabs)
}

func (r *duplicateTabsRenderer) buildAddTabButton() *widget.Button {
	addButton := widget.Button{
		Icon:       addIcon(r.duplicateTabs),
		Importance: widget.LowImportance,
		OnTapped: func() {
			newTab := r.duplicateTabs.OnAdded()
			r.duplicateTabs.Items = append(r.duplicateTabs.Items, newTab)
			r.Refresh()
			r.duplicateTabs.Select(newTab)
		},
	}
	return &addButton
}

func (r *duplicateTabsRenderer) buildOverflowTabsButton() (overflow *widget.Button) {
	overflow = &widget.Button{Icon: moreIcon(r.duplicateTabs), Importance: widget.LowImportance, OnTapped: func() {
		// Show pop up containing all tabs which did not fit in the tab bar

		itemLen, objLen := len(r.duplicateTabs.Items), len(r.bar.Objects[0].(*fyne.Container).Objects)
		items := make([]*fyne.MenuItem, 0, itemLen-objLen)
		for i := objLen; i < itemLen; i++ {
			index := i // capture
			// FIXME MenuItem doesn't support icons (#1752)
			// FIXME MenuItem can't show if it is the currently selected tab (#1753)
			items = append(items, fyne.NewMenuItem(r.duplicateTabs.Items[i].Text, func() {
				r.duplicateTabs.SelectIndex(index)
				if r.duplicateTabs.popupMenu != nil {
					r.duplicateTabs.popupMenu.Hide()
					r.duplicateTabs.popupMenu = nil
				}
			}))
		}

		r.duplicateTabs.popupMenu = buildPopUpMenu(r.duplicateTabs, overflow, items)
	}}

	return overflow
}

func (r *duplicateTabsRenderer) buildTabButtons(count int) *fyne.Container {
	buttons := &fyne.Container{}

	var iconPos buttonIconPosition
	if fyne.CurrentDevice().IsMobile() {
		cells := count
		if cells == 0 {
			cells = 1
		}
		if r.duplicateTabs.location == TabLocationTop || r.duplicateTabs.location == TabLocationBottom {
			buttons.Layout = layout.NewGridLayoutWithColumns(cells)
		} else {
			buttons.Layout = layout.NewGridLayoutWithRows(cells)
		}
		iconPos = buttonIconTop
	} else if r.duplicateTabs.location == TabLocationLeading || r.duplicateTabs.location == TabLocationTrailing {
		buttons.Layout = layout.NewVBoxLayout()
		iconPos = buttonIconTop
	} else {
		buttons.Layout = layout.NewHBoxLayout()
		iconPos = buttonIconInline
	}

	for i := 0; i < count; i++ {
		item := r.duplicateTabs.Items[i]
		button, ok := r.buttonCache[item]
		if !ok {
			button = &tabButton{
				onTapped: func() { r.duplicateTabs.Select(item) },
				onClosed: func() { r.duplicateTabs.Remove(item) },
			}
			r.buttonCache[item] = button
		}
		button.icon = item.Icon
		button.iconPosition = iconPos
		if i == r.duplicateTabs.current {
			button.importance = widget.HighImportance
		} else {
			button.importance = widget.MediumImportance
		}
		button.text = item.Text
		button.textAlignment = fyne.TextAlignCenter
		button.Refresh()
		buttons.Objects = append(buttons.Objects, button)
	}
	buttons.Objects = append(buttons.Objects, r.buildAddTabButton())
	return buttons
}

func (r *duplicateTabsRenderer) updateIndicator(animate bool) {
	if r.duplicateTabs.current < 0 {
		r.indicator.Hide()
		return
	}

	var selectedPos fyne.Position
	var selectedSize fyne.Size

	buttons := r.bar.Objects[0].(*fyne.Container).Objects
	if r.duplicateTabs.current >= len(buttons) {
		if a := r.action; a != nil {
			selectedPos = a.Position()
			selectedSize = a.Size()
		}
	} else {
		selected := buttons[r.duplicateTabs.current]
		selectedPos = selected.Position()
		selectedSize = selected.Size()
	}

	var indicatorPos fyne.Position
	var indicatorSize fyne.Size

	switch r.duplicateTabs.location {
	case TabLocationTop:
		indicatorPos = fyne.NewPos(selectedPos.X, r.bar.MinSize().Height)
		indicatorSize = fyne.NewSize(selectedSize.Width, theme.Padding())
	case TabLocationLeading:
		indicatorPos = fyne.NewPos(r.bar.MinSize().Width, selectedPos.Y)
		indicatorSize = fyne.NewSize(theme.Padding(), selectedSize.Height)
	case TabLocationBottom:
		indicatorPos = fyne.NewPos(selectedPos.X, r.bar.Position().Y-theme.Padding())
		indicatorSize = fyne.NewSize(selectedSize.Width, theme.Padding())
	case TabLocationTrailing:
		indicatorPos = fyne.NewPos(r.bar.Position().X-theme.Padding(), selectedPos.Y)
		indicatorSize = fyne.NewSize(theme.Padding(), selectedSize.Height)
	}

	r.moveIndicator(indicatorPos, indicatorSize, animate)
}

func (r *duplicateTabsRenderer) updateTabs(max int) {
	tabCount := len(r.duplicateTabs.Items)

	// Set overflow action
	if tabCount <= max {
		r.action.Hide()
		r.bar.Layout = layout.NewMaxLayout()
	} else {
		tabCount = max
		r.action.Show()

		// Set layout of tab bar containing tab buttons and overflow action
		if r.duplicateTabs.location == TabLocationLeading || r.duplicateTabs.location == TabLocationTrailing {
			r.bar.Layout = layout.NewBorderLayout(nil, r.action, nil, nil)
		} else {
			r.bar.Layout = layout.NewBorderLayout(nil, nil, nil, r.action)
		}
	}

	buttons := r.buildTabButtons(tabCount)

	r.bar.Objects = []fyne.CanvasObject{buttons}
	if a := r.action; a != nil {
		r.bar.Objects = append(r.bar.Objects, a)
	}

	r.bar.Refresh()
}
