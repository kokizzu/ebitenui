package widget

import (
	"fmt"
	img "image"

	"github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/input"

	"github.com/hajimehoshi/ebiten/v2"
)

type Container struct {
	definedParams       PanelParams
	computedParams      PanelParams
	AutoDisableChildren bool

	widgetOpts     []WidgetOpt
	layout         Layouter
	layoutDirty    bool
	relayoutParent bool
	validated      bool

	init     *MultiOnce
	widget   *Widget
	children []PreferredSizeLocateableWidget
}

type ContainerOpt func(c *Container)

type RemoveChildFunc func()

type ContainerOptions struct {
}

var ContainerOpts ContainerOptions

type PreferredSizeLocateableWidget interface {
	HasWidget
	PreferredSizer
	Locateable
	Validate()
}

func NewContainer(opts ...ContainerOpt) *Container {
	c := &Container{
		init: &MultiOnce{},
	}

	c.init.Append(c.createWidget)

	for _, o := range opts {
		o(c)
	}

	return c
}

func (o ContainerOptions) WidgetOpts(opts ...WidgetOpt) ContainerOpt {
	return func(c *Container) {
		c.widgetOpts = append(c.widgetOpts, opts...)
	}
}

// This will set the background image to the provided NineSlice. If this is set then
// we will automatically track that the UI has been hovered over for this container
// Use widget.WidgetOpts.TrackHover(false) to turn this off if desired.
func (o ContainerOptions) BackgroundImage(i *image.NineSlice) ContainerOpt {
	return func(c *Container) {
		c.definedParams.BackgroundImage = i
	}
}

func (o ContainerOptions) AutoDisableChildren() ContainerOpt {
	return func(c *Container) {
		c.AutoDisableChildren = true
	}
}

func (o ContainerOptions) Layout(layout Layouter) ContainerOpt {
	return func(c *Container) {
		c.layout = layout
	}
}

func (c *Container) AddChild(children ...PreferredSizeLocateableWidget) RemoveChildFunc {
	c.init.Do()

	for _, child := range children {
		if child == nil {
			panic("cannot add nil child")
		}

		child.GetWidget().parent = c.widget
		child.GetWidget().self = child

		if c.validated {
			child.Validate()
		}

		c.children = append(c.children, child)

		child.GetWidget().ContextMenuEvent.AddHandler(func(args interface{}) {
			if a, ok := args.(*WidgetContextMenuEventArgs); ok {
				c.GetWidget().FireContextMenuEvent(a.Widget, a.Location)
			}
		})
		child.GetWidget().FocusEvent.AddHandler(func(args interface{}) {
			if a, ok := args.(*WidgetFocusEventArgs); ok {
				c.GetWidget().FireFocusEvent(a.Widget, a.Focused, a.Location)
			}
		})
		child.GetWidget().ToolTipEvent.AddHandler(func(args interface{}) {
			if a, ok := args.(*WidgetToolTipEventArgs); ok {
				c.GetWidget().FireToolTipEvent(a.Window, a.Show)
			}
		})
		child.GetWidget().DragAndDropEvent.AddHandler(func(args interface{}) {
			if a, ok := args.(*WidgetDragAndDropEventArgs); ok {
				c.GetWidget().FireDragAndDropEvent(a.Window, a.Show, a.DnD)
			}
		})
	}
	c.RequestRelayout()
	c.relayoutParent = true
	return func() {
		for _, child := range children {
			c.RemoveChild(child)
		}
	}
}

func (c *Container) RemoveChild(child PreferredSizeLocateableWidget) {
	index := -1
	for i, ch := range c.children {
		if ch == child {
			index = i
			break
		}
	}

	if index < 0 {
		return
	}

	c.children = append(c.children[:index], c.children[index+1:]...)

	child.GetWidget().parent = nil

	if child.GetWidget().ToolTip != nil && child.GetWidget().ToolTip.window != nil {
		child.GetWidget().ToolTip.window.Close()
	}

	if child.GetWidget().DragAndDrop != nil && child.GetWidget().DragAndDrop.window != nil {
		child.GetWidget().DragAndDrop.window.Close()
	}

	if child.GetWidget().ContextMenuWindow != nil {
		child.GetWidget().ContextMenuWindow.Close()
	}
	c.RequestRelayout()
	c.relayoutParent = true
}

func (c *Container) RemoveChildren() {
	for i := range c.children {
		childWidget := c.children[i].GetWidget()
		childWidget.parent = nil

		if childWidget.ToolTip != nil && childWidget.ToolTip.window != nil {
			childWidget.ToolTip.window.Close()
		}

		if childWidget.DragAndDrop != nil && childWidget.DragAndDrop.window != nil {
			childWidget.DragAndDrop.window.Close()
		}

		if childWidget.ContextMenuWindow != nil {
			childWidget.ContextMenuWindow.Close()
		}
	}
	c.children = nil

	c.RequestRelayout()
	c.relayoutParent = true
}

func (c *Container) Children() []PreferredSizeLocateableWidget {
	return c.children
}

func (c *Container) RequestRelayout() {
	c.init.Do()

	c.layoutDirty = true

	for _, ch := range c.children {
		if r, ok := ch.(Relayoutable); ok {
			r.RequestRelayout()
		}
	}
}

func (c *Container) GetWidget() *Widget {
	c.init.Do()
	return c.widget
}

func (c *Container) PreferredSize() (int, int) {
	c.init.Do()
	w, h := 0, 0

	if !c.validated {
		c.Validate()
	}
	// Start with the background image min size if one is set
	if c.computedParams.BackgroundImage != nil {
		w, h = c.computedParams.BackgroundImage.MinSize()
	}

	// If the preferred layout for the children is greater than the background image
	// min size then use that
	if c.layout != nil {
		pW, pH := c.layout.PreferredSize(c.children)
		if pW > w {
			w = pW
		}
		if pH > h {
			h = pH
		}
	}

	// If the set MinHeight or MinWidth are greater than calculated, use that
	if c.widget != nil && h < c.widget.MinHeight {
		h = c.widget.MinHeight
	}
	if c.widget != nil && w < c.widget.MinWidth {
		w = c.widget.MinWidth
	}

	return w, h
}

func (c *Container) SetLocation(rect img.Rectangle) {
	c.init.Do()
	if c.widget.Rect != rect {
		c.widget.Rect = rect
		c.RequestRelayout()
	}
}

func (c *Container) Validate() {
	c.computedParams.BackgroundImage = c.definedParams.BackgroundImage

	for idx := range c.children {
		c.children[idx].Validate()
	}
	c.validated = true
}

func (c *Container) SetBackgroundImage(image *image.NineSlice) {
	c.definedParams.BackgroundImage = image
	c.Validate()
}

func (c *Container) Render(screen *ebiten.Image) {
	c.init.Do()

	if !c.widget.IsVisible() {
		return
	}

	if c.AutoDisableChildren {
		for _, ch := range c.children {
			ch.GetWidget().Disabled = c.widget.Disabled
		}
	}

	c.widget.Render(screen)

	c.doLayout()

	c.draw(screen)

	for _, ch := range c.children {
		if cr, ok := ch.(Renderer); ok {
			if !ch.GetWidget().IsVisible() {
				continue
			}
			cr.Render(screen)
		}
	}
}

func (c *Container) Update(updObj *UpdateObject) {
	c.init.Do()

	c.widget.Update(updObj)

	for _, ch := range c.children {
		if cu, ok := ch.(Updater); ok {
			cu.Update(updObj)
		}
	}

	if c.relayoutParent {
		updObj.RelayoutRequested = updObj.RelayoutRequested || true
		c.relayoutParent = false
	}
}

func (c *Container) doLayout() {
	if c.layout != nil && c.layoutDirty {
		if !c.validated {
			c.Validate()
		}
		c.layout.Layout(c.children, c.widget.Rect)
		c.layoutDirty = false
	}
}

func (c *Container) SetupInputLayer(def input.DeferredSetupInputLayerFunc) {
	c.init.Do()

	for idx, ch := range c.children {
		if v, ok := ch.(Focuser); ok {
			if ch.GetWidget().ElevateLayer {
				ch.GetWidget().ElevateToNewInputLayer(&input.Layer{
					DebugLabel: fmt.Sprintf("Container %p - Widget %d", &c, idx),
					EventTypes: input.LayerEventTypeAll,
					BlockLower: true,
					FullScreen: false,
					RectFunc: func() img.Rectangle {
						return v.GetWidget().Rect
					},
				})
			}
		}
		if il, ok := ch.(input.Layerer); ok {
			il.SetupInputLayer(def)
		}
	}
}

func (c *Container) draw(screen *ebiten.Image) {
	if c.computedParams.BackgroundImage != nil {
		c.computedParams.BackgroundImage.Draw(screen, c.widget.Rect.Dx(), c.widget.Rect.Dy(), c.widget.drawImageOptions)
	}
}

func (c *Container) createWidget() {
	c.widget = NewWidget(append([]WidgetOpt{WidgetOpts.TrackHover(c.definedParams.BackgroundImage != nil)}, c.widgetOpts...)...)
	c.widgetOpts = nil
	c.widget.self = c
}

func (c *Container) GetFocusers() []Focuser {
	var result []Focuser
	for _, child := range c.children {
		switch v := child.(type) {
		case Focuser:
			if widget, ok := v.(HasWidget); ok {
				if v.TabOrder() >= 0 && !widget.GetWidget().Disabled && widget.GetWidget().IsVisible() {
					result = append(result, v)
				}
			}
		case *Container:
			result = append(result, v.GetFocusers()...)
		case *FlipBook:
			result = append(result, v.GetFocusers()...)
		case *TabBook:
			result = append(result, v.container.GetFocusers()...)
		case *TabBookTab:
			result = append(result, v.GetFocusers()...)
		case *ScrollContainer:
			result = append(result, v.GetFocusers()...)
		case *TextArea:
			result = append(result, v.GetFocusers()...)
		}
	}
	return result
}

func (c *Container) GetDropTargets() []HasWidget {
	var result []HasWidget
	if c.GetWidget().drop != nil {
		result = append(result, c)
	}
	for _, child := range c.children {
		if v, ok := child.(Dropper); ok {
			result = append(result, v.GetDropTargets()...)
		} else if child.GetWidget().drop != nil {
			// If the Widget has 'drop' implemented then
			// we have to push them to the 'result' as
			// it means it has a handler for it
			result = append(result, child)
		}
	}

	return result
}

// WidgetAt implements WidgetLocator.
func (c *Container) WidgetAt(x int, y int) HasWidget {
	c.init.Do()

	p := img.Point{x, y}

	if !p.In(c.GetWidget().Rect) {
		return nil
	}

	for _, ch := range c.children {
		if wl, ok := ch.(Locater); ok {
			w := wl.WidgetAt(x, y)
			if w != nil {
				return w
			}

			continue
		}

		if p.In(ch.GetWidget().Rect) {
			return ch
		}
	}

	return c
}
