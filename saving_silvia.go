package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/qeedquan/go-media/sdl"
	"github.com/qeedquan/go-media/sdl/sdlgfx"
	"github.com/qeedquan/go-media/sdl/sdlimage"
	"github.com/qeedquan/go-media/sdl/sdlimage/sdlcolor"
	"github.com/qeedquan/go-media/sdl/sdlmixer"
)

var vec = [][2]float64{
	'n': {0, -1},
	's': {0, 1},
	'e': {1, 0},
	'w': {-1, 0},
}

var (
	window   *sdl.Window
	renderer *sdl.Renderer
	fps      sdlgfx.FPSManager

	conf struct {
		assets     string
		pref       string
		width      int
		height     int
		fullscreen bool
		invincible bool
		sound      bool
	}

	musics = make(map[string]*Music)
	images = make(map[string]*Image)

	ctls    []*sdl.GameController
	savePos int

	imageScene ImageScene
	playScene  PlayScene
	nextScene  Scene
	scene      Scene

	black *sdl.Texture
	pause bool
)

func main() {
	runtime.LockOSThread()
	parseFlags()
	initSDL()

	imageScene.Init("title")
	scene = &imageScene
	rc := uint(0)
	for {
		for {
			ev := sdl.PollEvent()
			if ev == nil {
				break
			}

			switch ev := ev.(type) {
			case sdl.QuitEvent:
				return

			case sdl.KeyDownEvent:
				switch ev.Sym {
				case sdl.K_ESCAPE:
					return
				case sdl.K_1:
					if savePos--; savePos < 0 {
						savePos = 10
					}
					sdl.Log("Save slot set to %d", savePos)
				case sdl.K_2:
					if savePos++; savePos > 10 {
						savePos = 0
					}
					sdl.Log("Save slot set to %d", savePos)
				case sdl.K_F2:
					saveGame(fmt.Sprintf("%d.sav", savePos))
				case sdl.K_F4:
					loadGame(fmt.Sprintf("%d.sav", savePos))
				case sdl.K_BACKSPACE:
					togglePause()
				case sdl.K_BACKQUOTE:
					toggleInvincible()
				}

			case sdl.ControllerButtonDownEvent:
				button := sdl.GameControllerButton(ev.Button)
				switch button {
				case sdl.CONTROLLER_BUTTON_LEFTSTICK:
					saveGame(fmt.Sprintf("%d.sav", savePos))
				case sdl.CONTROLLER_BUTTON_RIGHTSTICK:
					loadGame(fmt.Sprintf("%d.sav", savePos))
				case sdl.CONTROLLER_BUTTON_BACK:
					toggleInvincible()
				case sdl.CONTROLLER_BUTTON_START:
					togglePause()
				}

			case sdl.ControllerDeviceAddedEvent:
				mapControllers()
			}
		}

		sp, dx, dy := control()
		if nextScene != nil {
			scene, nextScene = nextScene, nil
		}

		if !pause {
			rc++
			scene.Update(sp, dx, dy)
			black.SetAlphaMod(0)
			sdlmixer.ResumeMusic()
		} else {
			black.SetAlphaMod(150)
			sdlmixer.PauseMusic()
		}

		renderer.SetDrawColor(sdlcolor.Black)
		renderer.Clear()
		scene.Render(rc)
		renderer.Copy(black, nil, nil)
		renderer.Present()

		fps.Delay()
	}
}

func togglePause() {
	if scene == &playScene {
		pause = !pause
	}
}

func toggleInvincible() {
	conf.invincible = !conf.invincible
	sdl.Log("Toggled invincibility %v", conf.invincible)
}

func control() (sp bool, dx, dy float64) {
	key := sdl.GetKeyboardState()
	sp = key[sdl.SCANCODE_SPACE] != 0 || key[sdl.SCANCODE_RETURN] != 0
	dx, dy = 0.0, 0.0
	switch {
	case key[sdl.SCANCODE_LEFT] != 0:
		dx = -1
	case key[sdl.SCANCODE_RIGHT] != 0:
		dx = 1
	}
	switch {
	case key[sdl.SCANCODE_UP] != 0:
		dy = -1
	case key[sdl.SCANCODE_DOWN] != 0:
		dy = 1
	}

	const threshold = 1000
	for _, c := range ctls {
		if c == nil {
			continue
		}

		switch {
		case c.Button(sdl.CONTROLLER_BUTTON_A) != 0,
			c.Button(sdl.CONTROLLER_BUTTON_B) != 0,
			c.Button(sdl.CONTROLLER_BUTTON_X) != 0,
			c.Button(sdl.CONTROLLER_BUTTON_Y) != 0:
			sp = true
		}

		switch {
		case c.Button(sdl.CONTROLLER_BUTTON_DPAD_LEFT) != 0,
			c.Axis(sdl.CONTROLLER_AXIS_LEFTX) < -threshold,
			c.Axis(sdl.CONTROLLER_AXIS_RIGHTX) < -threshold:
			dx = -1
		case c.Button(sdl.CONTROLLER_BUTTON_DPAD_RIGHT) != 0,
			c.Axis(sdl.CONTROLLER_AXIS_LEFTX) > threshold,
			c.Axis(sdl.CONTROLLER_AXIS_RIGHTX) > threshold:
			dx = 1
		}

		switch {
		case c.Button(sdl.CONTROLLER_BUTTON_DPAD_UP) != 0,
			c.Axis(sdl.CONTROLLER_AXIS_LEFTY) < -threshold,
			c.Axis(sdl.CONTROLLER_AXIS_RIGHTY) < -threshold:
			dy = -1
		case c.Button(sdl.CONTROLLER_BUTTON_DPAD_DOWN) != 0,
			c.Axis(sdl.CONTROLLER_AXIS_LEFTY) > threshold,
			c.Axis(sdl.CONTROLLER_AXIS_RIGHTY) > threshold:
			dy = 1
		}
	}

	return
}

func parseFlags() {
	conf.width = 256
	conf.height = 224
	conf.assets = filepath.Join(sdl.GetBasePath(), "assets")
	conf.pref = sdl.GetPrefPath("", "saving_silvia")
	flag.StringVar(&conf.assets, "assets", conf.assets, "assets directory")
	flag.StringVar(&conf.pref, "pref", conf.pref, "preference directory")
	flag.BoolVar(&conf.sound, "sound", true, "enable sound")
	flag.BoolVar(&conf.fullscreen, "fullscreen", false, "fullscreen mode")
	flag.BoolVar(&conf.invincible, "invincible", false, "be invincible")
	flag.Parse()
}

func initSDL() {
	err := sdl.Init(sdl.INIT_EVERYTHING &^ sdl.INIT_AUDIO)
	ck(err)

	err = sdl.InitSubSystem(sdl.INIT_AUDIO)
	ek(err)

	err = sdlmixer.OpenAudio(44100, sdl.AUDIO_S16, 2, 8192)
	ek(err)

	_, err = sdlmixer.Init(sdlmixer.INIT_OGG)
	ek(err)

	sdlmixer.AllocateChannels(128)

	sdl.SetHint(sdl.HINT_RENDER_SCALE_QUALITY, "best")
	wflag := sdl.WINDOW_RESIZABLE
	if conf.fullscreen {
		wflag |= sdl.WINDOW_FULLSCREEN_DESKTOP
	}
	window, renderer, err = sdl.CreateWindowAndRenderer(conf.width*3, conf.height*3, wflag)
	ck(err)

	sdl.ShowCursor(0)
	window.SetTitle("Saving Silvia")
	renderer.SetLogicalSize(conf.width, conf.height)
	renderer.Clear()
	renderer.Present()

	mapControllers()

	fps.Init()
	fps.SetRate(30)

	gray := image.NewGray(image.Rect(0, 0, conf.width, conf.height))
	black, err = sdlimage.LoadTextureImage(renderer, gray)
	ck(err)
}

func mapControllers() {
	for _, c := range ctls {
		if c != nil {
			c.Close()
		}
	}

	ctls = make([]*sdl.GameController, sdl.NumJoysticks())
	for i, _ := range ctls {
		if sdl.IsGameController(i) {
			var err error
			ctls[i], err = sdl.GameControllerOpen(i)
			ek(err)
		}
	}
}

func ck(err error) {
	if err != nil {
		sdl.LogCritical(sdl.LOG_CATEGORY_APPLICATION, "%v", err)
		sdl.ShowSimpleMessageBox(sdl.MESSAGEBOX_ERROR, "Error", err.Error(), window)
		os.Exit(1)
	}
}

func ek(err error) bool {
	if err != nil {
		sdl.LogError(sdl.LOG_CATEGORY_APPLICATION, "%v", err)
		return true
	}
	return false
}

type Music struct {
	*sdlmixer.Music
}

func loadMusic(name string) *Music {
	name = filepath.Join(conf.assets, "music", name+".ogg")
	if m, found := musics[name]; found {
		return m
	}

	mus, err := sdlmixer.LoadMUS(name)
	if ek(err) {
		return nil
	}

	musics[name] = &Music{mus}
	return musics[name]
}

func (m *Music) Play(n int) {
	if m == nil || !conf.sound {
		return
	}
	m.Music.Play(n)
}

type Image struct {
	*sdl.Texture
	width  int
	height int
	flip   sdl.RendererFlip
}

func loadImage(name string) (*Image, error) {
	name = filepath.Join(conf.assets, name+".png")
	if m, found := images[name]; found {
		return m, nil
	}

	texture, err := sdlimage.LoadTextureFile(renderer, name)
	if err != nil {
		return nil, err
	}

	_, _, width, height, err := texture.Query()
	ck(err)

	m := &Image{
		Texture: texture,
		width:   width,
		height:  height,
		flip:    sdl.FLIP_NONE,
	}
	images[name] = m
	return m, nil
}

func (m *Image) Flip() *Image {
	p := *m
	p.flip = sdl.FLIP_HORIZONTAL
	return &p
}

func (m *Image) Blit(x, y int) {
	renderer.CopyEx(m.Texture, nil, &sdl.Rect{int32(x), int32(y), int32(m.width), int32(m.height)}, 0, nil, m.flip)
}

type Scene interface {
	Update(sp bool, dx, dy float64)
	Render(rc uint)
}

type ImageScene struct {
	kind  string
	input bool
}

func (p *ImageScene) Init(kind string) {
	p.kind = kind
	p.input = false

	music := "title"
	switch kind {
	case "death":
		music = kind
	case "story":
		music = ""
	}

	if music != "" {
		mus := loadMusic(music)
		mus.Play(-1)
	}
}

func (p *ImageScene) Update(sp bool, dx, dy float64) {
	p.input = p.input || !sp
	if p.input && sp {
		switch p.kind {
		case "title":
			p.Init("story")

		case "story":
			playScene.Init()
			nextScene = &playScene

		case "victory", "death":
			p.Init("title")
		}
	}
}

func (p *ImageScene) Render(uint) {
	image, err := loadImage(filepath.Join("screens", p.kind))
	ck(err)
	image.Blit(0, 0)
}

type PlayScene struct {
	player  *Sprite
	tiles   [][]*Tile
	maps    [][]*Map
	current *Map
	sprites []*Sprite
	keys    map[string]bool
	music   string
	sword   struct {
		counter int
		allow   bool
		x, y    float64
	}
	state struct {
		Player struct {
			X, Y float64
			Dir  rune
		}
		Map struct {
			X, Y int
		}
		Keys map[string]bool
	}
}

func (p *PlayScene) Init() {
	p.InitEx(12.5, 7.5, 's', 0, 5, nil)
}

func (p *PlayScene) InitEx(px, py float64, pdir rune, mx, my int, keys map[string]bool) {
	p.player = newSpriteEx("player", px, py, pdir)
	p.tiles = nil
	p.maps = loadMaps()
	if mx >= len(p.maps) || my >= len(p.maps[0]) {
		mx, my = 0, 5
	}
	p.current = p.maps[mx][my]
	p.sprites = nil
	if p.keys = keys; p.keys == nil {
		p.keys = make(map[string]bool)
	}
	p.music = ""

	p.sword.counter = -1
	p.sword.x, p.sword.y = 0, 0
	p.sword.allow = false

	p.state.Player.X = px
	p.state.Player.Y = py
	p.state.Player.Dir = pdir
	p.state.Map.X = mx
	p.state.Map.Y = my
	p.state.Keys = p.keys
}

func (p *PlayScene) MarshalJSON() ([]byte, error) {
	return json.Marshal(&p.state)
}

func (p *PlayScene) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, &p.state)
}

func (p *PlayScene) initMap() {
	store := loadTiles()
	tiles := make([][]*Tile, 16)
	for i := range tiles {
		tiles[i] = make([]*Tile, 14)
	}
	maps := p.maps[p.current.col][p.current.row]
	sprites := []*Sprite{p.player}

	for y := range tiles[0] {
		for x := range tiles {
			id := maps.tiles[x][y]
			if strings.ContainsRune("!@#$%^&*{}?", rune(id)) {
				sid := string(id)
				switch id {
				case '!':
					sid = "slime"
				case '@':
					sid = "ogre"
				case '#':
					sid = "bat"
				case '$':
					sid = "firespit"
				case '%':
					sid = "troll"
				case '^':
					sid = "bug"
				case '&':
					sid = "wolf"
				case '*':
					sid = "princess"
				case '{':
					sid = "redkey"
				case '}':
					sid = "greenkey"
				case '?':
					sid = "bluekey"
				}

				sprites = append(sprites, newSprite(sid, float64(x)+0.5, float64(y)+0.5))
				if maps.music == "overworld" {
					id = 'L'
				} else {
					id = 'I'
				}
			}

			if t := store[string(id)]; t != nil {
				tiles[x][y] = &Tile{t.Image, string(id), t.pass}
			}
		}
	}

	if maps.music != p.music {
		mus := loadMusic(maps.music)
		mus.Play(-1)
		p.music = maps.music
	}

	p.tiles = tiles
	p.sprites = sprites
}

func (p *PlayScene) Update(sp bool, dx, dy float64) {
	if p.tiles == nil {
		p.initMap()
	}

	if !sp {
		p.sword.allow = true
	}

	if p.sword.allow && sp && p.sword.counter < 0 {
		p.sword.counter = 6
		p.sword.x = p.player.x + vec[p.player.dir][0]
		p.sword.y = p.player.y + vec[p.player.dir][1]
	}

	if p.sword.counter < 0 {
		p.player.dx = dx * 0.15
		p.player.dy = dy * 0.15
	}

	var sprites []*Sprite
	for _, s := range p.sprites {
		s.Update(p, p.player)
		if !s.dead {
			sprites = append(sprites, s)
		}
	}
	p.sprites = sprites

	if p.sword.counter > -1 {
		p.sword.counter--
	}

	px := p.player.x
	py := p.player.y
	mx := p.current.col
	my := p.current.row
	nx := mx
	ny := my
	if px < 0.5 {
		nx--
		p.player.x = 15.3
	} else if px > 15.5 {
		nx++
		p.player.x = 0.8
	}
	if py < 0.5 {
		ny--
		p.player.y = 13.3
	} else if py > 13.5 {
		ny++
		p.player.y = 0.8
	}

	if nx != mx || ny != my {
		p.tiles = nil
		p.current = p.maps[nx][ny]
		p.initMap()

		p.state.Player.X = p.player.x
		p.state.Player.Y = p.player.y
		p.state.Player.Dir = p.player.dir
		p.state.Map.X = nx
		p.state.Map.Y = ny
	}

	if p.sword.counter > -1 {
		p.sword.counter--
	}
}

func (p *PlayScene) Render(rc uint) {
	for y := range p.tiles[0] {
		for x := range p.tiles {
			if p.tiles[x][y] != nil {
				p.tiles[x][y].Blit(x*16, y*16)
			}
		}
	}

	for _, s := range p.sprites {
		s.Render(rc)
	}

	if p.sword.counter >= 0 {
		x := int32(16*p.sword.x) - 8
		y := int32(16*p.sword.y) - 8
		w := int32(16)
		h := int32(16)
		if strings.ContainsRune("ns", p.player.dir) {
			x += 6
			w = 4
		} else {
			y += 6
			h = 4
		}
		renderer.SetDrawColor(sdlcolor.White)
		renderer.FillRect(&sdl.Rect{x, y, w, h})
	}
}

type Sprite struct {
	kind     string
	dead     bool
	spit     bool
	moving   bool
	x, y     float64
	dx, dy   float64
	ax, ay   float64
	vary     bool
	collided bool
	dir      rune
	counter  uint
	images   map[string]*Image
}

func newSprite(kind string, x, y float64) *Sprite {
	return newSpriteEx(kind, x, y, 's')
}

func newSpriteEx(kind string, x, y float64, dir rune) *Sprite {
	return &Sprite{
		kind:   kind,
		x:      x,
		y:      y,
		dir:    dir,
		images: loadSprites(kind),
	}
}

func lsh(kind string, sprites map[string]*Image, primary, secondary string, flip bool) *Image {
	image, err := loadImage(filepath.Join("sprites", kind, primary))
	if err != nil {
		image = sprites[secondary]
		if flip {
			image = image.Flip()
		}
	}
	return image
}

func loadSprites(kind string) map[string]*Image {
	image, err := loadImage(filepath.Join("sprites", kind, "s0"))
	ck(err)

	sprites := make(map[string]*Image)
	sprites["s0"] = image
	sprites["s1"] = lsh(kind, sprites, "s1", "s0", true)
	sprites["n0"] = lsh(kind, sprites, "n0", "s0", false)
	sprites["n1"] = lsh(kind, sprites, "n1", "s1", false)
	sprites["e0"] = lsh(kind, sprites, "e0", "s0", false)
	sprites["e1"] = lsh(kind, sprites, "e1", "s1", false)
	sprites["w0"] = lsh(kind, sprites, "w0", "e0", true)
	sprites["w1"] = lsh(kind, sprites, "w1", "e1", true)
	return sprites
}

func dist(ax, ay, bx, by float64) float64 {
	dx := ax - bx
	dy := ay - by
	return math.Sqrt(dx*dx + dy*dy)
}

func sign(x float64) float64 {
	switch {
	case x == 0:
		return 0
	case x < 0:
		return -1
	default:
		return 1
	}
}

func (s *Sprite) Update(scene *PlayScene, player *Sprite) {
	s.moving = false
	if scene.sword.counter >= 0 &&
		s.kind != "player" && s.kind != "redkey" &&
		s.kind != "bluekey" && s.kind != "greenkey" {
		dx := s.x - scene.sword.x
		dy := s.y - scene.sword.y
		if math.Sqrt(dx*dx+dy*dy) < 1 {
			s.dead = true
			return
		}
	}

	switch {
	case strings.HasSuffix(s.kind, "key"):
		if dist(player.x, player.y, s.x, s.y) < 1 {
			s.dead = true
			scene.keys[s.kind] = true
		}

	case s.kind == "player":

	default:
		if dist(s.x, s.y, player.x, player.y) < 1 && !conf.invincible {
			imageScene.Init("death")
			nextScene = &imageScene
			return
		}

		c := s.counter
		switch s.kind {
		case "slime", "bug":
			n := c % 120
			if c < 60 {
				if n == 0 {
					const dirs = "nsew"
					i := rand.Intn(len(dirs))
					s.dir = rune(dirs[i])
				}

				s.dx = vec[s.dir][0] * .1
				s.dy = vec[s.dir][1] * .1
			}

		case "ogre", "wolf":
			s.dx = sign(player.x-s.x) * 0.08
			s.dy = sign(player.y-s.y) * 0.08

		case "firespit":
			n := c % 150
			if n < 40 {
				s.spit = true
				if n == 20 {
					spr := newSprite("fireball", s.x, s.y)
					rad := rand.Float64() * 2 * math.Pi
					s.ax = math.Cos(rad) * .2
					s.ay = math.Sin(rad) * .2
					scene.sprites = append(scene.sprites, spr)
				}
			} else {
				s.spit = false
			}

		case "fireball":
			if c > 3.5*30 {
				s.dead = true
			}

		case "bat":
			n := c % 90
			if n == 0 {
				rad := rand.Float64() * 2 * math.Pi
				s.ax = math.Cos(rad) * .1
				s.ay = math.Sin(rad) * .1
			}

		case "troll":
			if !s.vary {
				s.vary = rand.Float64() < .5
			}
			if s.collided {
				s.vary = !s.vary
			}

			if s.vary {
				s.dx = .1
			} else {
				s.dx = -.1
			}
		}
	}

	s.counter++
	nx := s.x + s.dx + s.ax
	ny := s.y + s.dy + s.ay
	outside := nx < 0 || nx >= 16 || ny < 0 || ny >= 14
	if outside && s.kind == "fireball" {
		s.dead = true
		return
	}

	s.moving = s.dx != 0 || s.dy != 0 || s.ax != 0 || s.ay != 0
	s.collided = false
	if !outside && s.moving {
		tile := scene.tiles[int(nx)][int(ny)]
		if tile == nil {
			return
		}
		pass := tile.pass
		if !pass && s.kind == "player" && strings.ContainsRune("123", rune(tile.id[0])) {
			switch tile.id[0] {
			case '1':
				if scene.keys["redkey"] {
					pass = true
				}
			case '2':
				if scene.keys["greenkey"] {
					pass = true
				}
			case '3':
				if scene.keys["bluekey"] {
					imageScene.Init("victory")
					nextScene = &imageScene
				}
			}
		}

		if pass {
			switch {
			case ny > s.y:
				s.dir = 's'
			case ny < s.y:
				s.dir = 'n'
			case nx < s.x:
				s.dir = 'w'
			default:
				s.dir = 'e'
			}

			s.x, s.y = nx, ny
			s.moving = true
		} else {
			s.collided = true
		}
	} else {
		s.collided = true
	}

	s.dx, s.dy = 0, 0
}

func (s *Sprite) Render(rc uint) {
	n := "0"
	if s.spit || (s.moving && (rc/5)%2 == 0) {
		n = "1"
	}
	image := s.images[string(s.dir)+n]
	image.Blit(int(s.x*16-8), int(s.y*16-8))
}

type Map struct {
	row, col int
	music    string
	tiles    [][]int
}

func loadMaps() [][]*Map {
	maps := make([][]*Map, 10)
	for i := range maps {
		maps[i] = make([]*Map, 6)
	}

	glob := filepath.Join(conf.assets, "maps", "*")
	files, err := filepath.Glob(glob)
	ck(err)

	for _, file := range files {
		base := filepath.Base(file)
		ext := filepath.Ext(base)
		base = base[:len(base)-len(ext)]

		fields := strings.Split(base, "-")
		if len(fields) != 3 {
			continue
		}

		col, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		row, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		col, row = col-1, row-1

		music := "overworld"
		if fields[2] != "o" {
			music = "dungeon"
		}

		fd, err := os.Open(file)
		ck(err)
		defer fd.Close()
		scanner := bufio.NewScanner(fd)

		tiles := make([][]int, 16)
		for i := range tiles {
			tiles[i] = make([]int, 14)
		}
		for y := range tiles[0] {
			if !scanner.Scan() {
				break
			}
			line := scanner.Text()

			for x := range tiles {
				tiles[x][y] = int(line[x])
			}
		}

		maps[col][row] = &Map{
			col:   col,
			row:   row,
			music: music,
			tiles: tiles,
		}
	}

	return maps
}

type Tile struct {
	*Image
	id   string
	pass bool
}

func loadTiles() map[string]*Tile {
	glob := filepath.Join(conf.assets, "tiles", "*")
	files, err := filepath.Glob(glob)
	ck(err)

	tiles := make(map[string]*Tile)
	for _, file := range files {
		base := filepath.Base(file)
		ext := filepath.Ext(base)
		base = base[:len(base)-len(ext)]

		id := strings.ToUpper(base)
		if len(id) == 0 {
			continue
		}
		pass := false
		if strings.ContainsRune("LIGFM", rune(id[0])) {
			pass = true
		}

		image, err := loadImage(filepath.Join("tiles", base))
		ck(err)

		tiles[id] = &Tile{
			Image: image,
			id:    id,
			pass:  pass,
		}
	}
	return tiles
}

func saveGame(name string) {
	if scene != &playScene {
		ek(fmt.Errorf("Save is only possible in-game"))
		return
	}

	name = filepath.Join(conf.pref, name)
	buf, err := json.MarshalIndent(&playScene, "", "\t")
	if ek(err) {
		return
	}

	f, err := os.Create(name)
	if ek(err) {
		return
	}
	_, err = f.Write(buf)
	ek(err)

	err = f.Close()
	ek(err)

	if err == nil {
		sdl.Log("Saved game to %s successfully", name)
	}
}

func loadGame(name string) {
	name = filepath.Join(conf.pref, name)
	buf, err := ioutil.ReadFile(name)
	if ek(err) {
		return
	}

	var p PlayScene
	err = json.Unmarshal(buf, &p.state)
	if ek(err) {
		return
	}

	s := &p.state
	playScene.InitEx(s.Player.X, s.Player.Y, s.Player.Dir, s.Map.X, s.Map.Y, s.Keys)
	nextScene = &playScene
	pause = false
	sdl.Log("Loaded game from %s successfully", name)
}
