package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed index.html
var indexHTML embed.FS

var BoundaryWall = Wall{}

func characterShoot(level *Level, character Entity, dx float64, dy float64) {
	bullet := &Bullet{
		id:       character.Id(),
		active:   true,
		name:     "bullet",
		x:        character.X() + (float64(character.Width()) / 4),
		y:        character.Y() + (float64(character.Height()) / 4),
		width:    8,
		height:   8,
		speed:    4,
		friction: 0.999,
	}
	bullet.SetVelocity(dx*bullet.Speed(), dy*bullet.Speed())
	level.entities.Append(bullet)
}

func (level *Level) tick() {
	t := time.Now()
	level.entities.Iterate(func(entity Entity) {
		// Apply velocity to position
		newX := entity.X() + entity.VelocityX()
		newY := entity.Y() + entity.VelocityY()

		// Boundary checks
		hitWall := false
		if newX < 0 {
			hitWall = true
			newX = 0
		} else if newX+float64(entity.Width()) > float64(level.width) {
			hitWall = true
			newX = float64(level.width) - float64(entity.Width())
		}

		if newY < 0 {
			hitWall = true
			newY = 0
		} else if newY+float64(entity.Height()) > float64(level.height) {
			hitWall = true
			newY = float64(level.height) - float64(entity.Height())
		}

		// Boundary checks
		blocked := false
		if hitWall {
			blocked = entity.HandleCollision(level, &BoundaryWall).Blocked
		}

		// Collision checks
		level.entities.Iterate(func(other Entity) {
			if entity.Id() != other.Id() {
				top, right, bottom, left := other.BoundingBox()
				if newX < right && newX+float64(entity.Width()) > left &&
					newY < bottom && newY+float64(entity.Height()) > top {
					blocked = blocked || entity.HandleCollision(level, other).Blocked
				}
			}
		})

		if !blocked {
			entity.SetPosition(newX, newY)
		}

		// Apply friction
		entity.SetVelocity(entity.VelocityX()*entity.Friction(), entity.VelocityY()*entity.Friction())
	})
	level.entities.RemoveInactive()
	level.tickTime = float64(time.Since(t).Milliseconds())
}

type Game struct {
	id    string
	level *Level
}

type EntityList struct {
	mu       sync.RWMutex
	entities []Entity
}

func (el *EntityList) Append(entity Entity) {
	el.mu.Lock()
	defer el.mu.Unlock()
	el.entities = append(el.entities, entity)
}

func (el *EntityList) Iterate(f func(entity Entity)) {
	el.mu.RLock()
	defer el.mu.RUnlock()
	for _, entity := range el.entities {
		f(entity)
	}
}

func (el *EntityList) RemoveInactive() {
	el.mu.Lock()
	defer el.mu.Unlock()

	for i, entity := range el.entities {
		if !entity.Active() {
			el.entities = append(el.entities[:i], el.entities[i+1:]...)
			return
		}
	}
}

func (el *EntityList) SpawnEntity(level *Level, entity Entity) {
	el.mu.Lock()
	defer el.mu.Unlock()

	startTime := time.Now()

	for {
		// Generate random position within level bounds
		x := rand.Float64() * float64(level.width-entity.Width())
		y := rand.Float64() * float64(level.height-entity.Height())

		entity.SetPosition(x, y)

		// Check for intersection with existing entities
		intersects := false
		for _, other := range el.entities {
			if entity.Id() != other.Id() {
				top, right, bottom, left := other.BoundingBox()
				entTop, entRight, entBottom, entLeft := entity.BoundingBox()
				if entLeft < right && entRight > left && entTop < bottom && entBottom > top {
					intersects = true
					break
				}
			}
		}

		if !intersects {
			// Place the entity if no intersection
			el.entities = append(el.entities, entity)
			return
		}

		// Check if 100ms has passed
		if time.Since(startTime) > 100*time.Millisecond {
			// Unlock and wait for 500ms before trying again
			el.mu.Unlock()
			time.Sleep(500 * time.Millisecond)
			el.mu.Lock()
			startTime = time.Now()
		}
	}
}

type Level struct {
	width    int
	height   int
	entities EntityList
	tickTime float64
}

type EntityType int

const (
	WallType EntityType = iota
	CharacterType
	BulletType
)

func (e EntityType) String() string {
	switch e {
	case WallType:
		return "wall"
	case CharacterType:
		return "character"
	case BulletType:
		return "bullet"
	default:
		return "unknown"
	}
}

type CollisionResult struct {
	Blocked bool
}

type Entity interface {
	Id() string
	Active() bool
	SetActive(bool)
	Type() EntityType
	Name() string
	X() float64
	Y() float64
	Width() int
	Height() int
	BoundingBox() (float64, float64, float64, float64)
	VelocityX() float64
	VelocityY() float64
	SetPosition(x, y float64)
	SetVelocity(velocityX, velocityY float64)
	Speed() float64
	Friction() float64
	HandleCollision(*Level, Entity) CollisionResult
}

type Wall struct {
	x      float64
	y      float64
	width  int
	height int
}

func (w *Wall) Id() string {
	return "wall"
}

func (w *Wall) Active() bool {
	return true
}

func (w *Wall) SetActive(b bool) {}

func (w *Wall) Type() EntityType {
	return WallType
}

func (w *Wall) Name() string {
	return "wall"
}

func (w *Wall) X() float64 {
	return w.x
}

func (w *Wall) Y() float64 {
	return w.y
}

func (w *Wall) Width() int {
	return w.width
}

func (w *Wall) Height() int {
	return w.height
}

func (w *Wall) BoundingBox() (float64, float64, float64, float64) {
	top := w.y
	right := w.x + float64(w.width)
	bottom := w.y + float64(w.height)
	left := w.x
	return top, right, bottom, left
}

func (w *Wall) VelocityX() float64 {
	return 0
}

func (w *Wall) VelocityY() float64 {
	return 0
}

func (w *Wall) SetPosition(x, y float64) {}

func (w *Wall) SetVelocity(velocityX, velocityY float64) {}

func (w *Wall) Speed() float64 {
	return 0
}

func (w *Wall) Friction() float64 {
	return 0
}

func (w *Wall) HandleCollision(level *Level, entity Entity) CollisionResult {
	return CollisionResult{
		Blocked: false,
	}
}

type Character struct {
	id        string
	active    bool
	score     int
	lastHit   float64
	name      string
	x         float64
	y         float64
	width     int
	height    int
	velocityX float64
	velocityY float64
	speed     float64
	friction  float64
}

func (c *Character) Id() string {
	return c.id
}

func (c *Character) Active() bool {
	return c.active
}

func (c *Character) AdjScore(x int) {
	c.score = c.score + x
}

func (c *Character) AdjLastHit(x float64) {
	c.lastHit = x
}

func (c *Character) SetActive(b bool) {
	c.active = b
}

func (c *Character) Type() EntityType {
	return CharacterType
}

func (c *Character) Name() string {
	return c.name
}

func (c *Character) X() float64 {
	return c.x
}

func (c *Character) Y() float64 {
	return c.y
}

func (c *Character) Width() int {
	return c.width
}

func (c *Character) Height() int {
	return c.height
}

func (c *Character) BoundingBox() (float64, float64, float64, float64) {
	top := c.y
	right := c.x + float64(c.width)
	bottom := c.y + float64(c.height)
	left := c.x
	return top, right, bottom, left
}

func (c *Character) VelocityX() float64 {
	return c.velocityX
}

func (c *Character) VelocityY() float64 {
	return c.velocityY
}

func (c *Character) SetPosition(x, y float64) {
	c.x = x
	c.y = y
}

func (c *Character) SetVelocity(velocityX, velocityY float64) {
	c.velocityX = velocityX
	c.velocityY = velocityY
}

func (c *Character) Speed() float64 {
	return c.speed
}

func (c *Character) Friction() float64 {
	return c.friction
}

func (c *Character) HandleCollision(level *Level, entity Entity) CollisionResult {
	if entity.Type() == CharacterType || entity.Type() == WallType {
		return CollisionResult{
			Blocked: true,
		}
	}

	return CollisionResult{
		Blocked: false,
	}
}

type Bullet struct {
	id        string
	active    bool
	name      string
	x         float64
	y         float64
	width     int
	height    int
	velocityX float64
	velocityY float64
	speed     float64
	friction  float64
}

func (b *Bullet) Id() string {
	return b.id
}

func (b *Bullet) Active() bool {
	return b.active
}

func (b *Bullet) SetActive(_b bool) {
	b.active = _b
}

func (b *Bullet) Type() EntityType {
	return BulletType
}

func (b *Bullet) Name() string {
	return b.name
}

func (b *Bullet) X() float64 {
	return b.x
}

func (b *Bullet) Y() float64 {
	return b.y
}

func (b *Bullet) Width() int {
	return b.width
}

func (b *Bullet) Height() int {
	return b.height
}

func (b *Bullet) BoundingBox() (float64, float64, float64, float64) {
	top := b.y
	right := b.x + float64(b.width)
	bottom := b.y + float64(b.height)
	left := b.x
	return top, right, bottom, left
}

func (b *Bullet) VelocityX() float64 {
	return b.velocityX
}

func (b *Bullet) VelocityY() float64 {
	return b.velocityY
}

func (b *Bullet) Speed() float64 {
	return b.speed
}

func (b *Bullet) SetPosition(x, y float64) {
	b.x = x
	b.y = y
}

func (b *Bullet) SetVelocity(velocityX, velocityY float64) {
	b.velocityX = velocityX
	b.velocityY = velocityY
}

func (b *Bullet) Friction() float64 {
	return b.friction
}

func (b *Bullet) HandleCollision(level *Level, entity Entity) CollisionResult {
	if entity.Type() == CharacterType && entity.Id() != b.id {
		b.SetActive(false)

		entity.(*Character).AdjScore(-1)
		entity.(*Character).AdjLastHit(float64(time.Now().UnixMilli()))
		level.entities.Iterate(func(entity Entity) {
			if entity.Type() == CharacterType && entity.Id() == b.id {
				entity.(*Character).AdjScore(1)
			}
		})
		return CollisionResult{
			Blocked: false,
		}
	} else if entity.Type() == WallType {
		b.SetActive(false)
		return CollisionResult{
			Blocked: true,
		}
	}

	return CollisionResult{
		Blocked: false,
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // might wanna check this in some cases
	},
}

type ControlMessage struct {
	Game string `json:"game"`
	Name string `json:"name"`
}

type Action struct {
	Type      string     `json:"type"`
	Direction *Direction `json:"direction,omitempty"`
	Shoot     *Shoot     `json:"shoot,omitempty"`
}

type Direction struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Shoot struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (l *Level) toJSON(user Entity) ([]byte, error) {
	type EntitySlim struct {
		Type    string  `json:"type"`
		You     bool    `json:"you"`
		Id      string  `json:"id"`
		Score   int     `json:"score"`
		LastHit float64 `json:"lastHit"`
		Name    string  `json:"name"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
		Width   int     `json:"width"`
		Height  int     `json:"height"`
	}

	entities := []EntitySlim{}
	l.entities.Iterate(func(entity Entity) {
		entitySlim := EntitySlim{
			Type:   entity.Type().String(),
			You:    entity.Id() == user.Id(),
			Id:     entity.Id(),
			Name:   entity.Name(),
			X:      entity.X(),
			Y:      entity.Y(),
			Width:  entity.Width(),
			Height: entity.Height(),
		}
		if entity.Type() == CharacterType {
			entitySlim.Score = entity.(*Character).score
			entitySlim.LastHit = entity.(*Character).lastHit
		}
		entities = append(entities, entitySlim)
	})
	return json.Marshal(struct {
		Time     int          `json:"timeMs"`
		TickTime float64      `json:"tickTimeMs"`
		Width    int          `json:"width"`
		Height   int          `json:"height"`
		Entities []EntitySlim `json:"entities"`
	}{
		Time:     int(time.Now().UnixMilli()),
		TickTime: l.tickTime,
		Width:    l.width,
		Height:   l.height,
		Entities: entities,
	})
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer ws.Close()

	var controlMsg ControlMessage
	err = ws.ReadJSON(&controlMsg)
	if err != nil {
		fmt.Println(err)
		return
	}

	game, exists := games[controlMsg.Game]
	if !exists {
		fmt.Println("Game not found:", controlMsg.Game)
		return
	}

	character := &Character{
		id:       fmt.Sprintf("%d", rand.Int()),
		active:   true,
		name:     controlMsg.Name,
		x:        8,
		y:        8,
		width:    16,
		height:   16,
		friction: 0.9,
		speed:    2,
	}
	game.level.entities.SpawnEntity(game.level, character)

	go func() {
		ticker := time.NewTicker(time.Second / 60)
		defer ticker.Stop()

		for range ticker.C {
			data, err := game.level.toJSON(character)
			if err != nil {
				fmt.Println("Error serializing level:", err)
				break
			}
			err = ws.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				fmt.Println("Error sending game update:", err)

				character.SetActive(false)
				break
			}
		}
	}()

	for {
		var action Action
		err := ws.ReadJSON(&action)
		if err != nil {
			fmt.Println("Error reading control message:", err)
			break
		}

		switch action.Type {
		case "direction":
			if action.Direction != nil {
				dir := action.Direction
				length := math.Sqrt(dir.X*dir.X + dir.Y*dir.Y)
				if length > 0 {
					dir.X /= length
					dir.Y /= length
				}
				character.SetVelocity(dir.X*character.Speed(), dir.Y*character.Speed())
			}
		case "shoot":
			if action.Shoot != nil {
				dx := action.Shoot.X - character.X()
				dy := action.Shoot.Y - character.Y()
				length := math.Sqrt(dx*dx + dy*dy)
				if length > 0 {
					dx /= length
					dy /= length
				}
				characterShoot(game.level, character, dx, dy)
			}
		}
	}
}

var games map[string]Game

func gameLoop(game *Game) {
	for {
		time.Sleep(time.Second / 120)
		game.level.tick()
	}
}

func main() {
	level := &Level{
		width:  800,
		height: 800,
		entities: EntityList{
			mu:       sync.RWMutex{},
			entities: []Entity{},
		},
	}
	level.entities.Append(&Wall{
		x:      600,
		y:      100,
		width:  2,
		height: 400,
	})
	level.entities.Append(&Wall{
		x:      100,
		y:      600,
		width:  400,
		height: 2,
	})
	level.entities.Append(&Wall{
		x:      200,
		y:      200,
		width:  350,
		height: 2,
	})

	game := &Game{id: "test", level: level}
	go gameLoop(game)

	games = map[string]Game{"test": *game}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.Handle("/", http.FileServer(http.FS(indexHTML)))
	http.HandleFunc("/ws", handleConnections)
	log.Println("listening on", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		fmt.Println("ListenAndServe:", err)
	}
}
