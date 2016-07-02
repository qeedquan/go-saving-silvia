import pygame
import os
import random
import math

def dist(a, b):
	dx = a.x - b.x
	dy = a.y - b.y
	return (dx ** 2 + dy ** 2) ** .5

_images = {}
def get_image(path):
	img = _images.get(path)
	if img == None:
		rpath = path.replace('/', os.sep)
		if os.path.exists(rpath):
			img = pygame.image.load(rpath)
		else:
			img = 'candy'
		_images[path] = img
	if img == 'candy': return None
	return img

def make_grid(w, h):
	output = []
	for i in range(w):
		output.append([0]* h)
	return output

def load_tiles():
	tiles = {}
	for file in os.listdir('tiles'):
		
		id = file.split('.')[0].upper()
		pas = id in 'LIGFM'
		img = get_image('tiles/' + file)
		
		t = SC()
		t.id = id
		t.passable = pas
		t.img = img
		tiles[id] = t
	return tiles

vec = {
	'n': (0, -1),
	's': (0, 1),
	'e': (1, 0),
	'w': (-1, 0) }

def sign(x):
	if x == 0: return 0
	if x < 0: return -1
	return 1

def lsh(type, lookup, prim, second, flip):
	img = get_image('sprites/' + type + '/' + prim + '.png')
	if img == None:
		img = lookup[second]
		if flip:
			img = pygame.transform.flip(img, True, False)
	return img

def load_sprites(type):
	output  = {}
	output['s0'] = get_image('sprites/' + type + '/s0.png')
	output['s1'] = lsh(type, output, 's1', 's0', True)
	output['n0'] = lsh(type, output, 'n0', 's0', False)
	output['n1'] = lsh(type, output, 'n1', 's1', False)
	output['e0'] = lsh(type, output, 'e0', 's0', False)
	output['e1'] = lsh(type, output, 'e1', 's1', False)
	output['w0'] = lsh(type, output, 'w0', 'e0', True)
	output['w1'] = lsh(type, output, 'w1', 'e1', True)
	return output

class Sprite:
	def __init__(self, type, x, y):
		self.type = type
		self.dead = False
		self.spit = False
		self.moving = False
		self.x = x
		self.y = y
		self.dx = 0
		self.dy = 0
		self.collided = False
		self.dir = 's'
		self.pdx = 0
		self.pdy = 0
		self.var = None
		self.images = load_sprites(type)
		self.counter = 0
		
	
	def update(self, scene, player):

		self.moving = False
		if scene.swordcounter >= 0 and not (self.type in ('player', 'redkey', 'bluekey', 'greenkey')):
			dx = self.x - scene.swordx
			dy = self.y - scene.swordy
			d = (dx ** 2 + dy ** 2) ** .5
			
			if d < 1:
				self.dead = True
				return
		
		t = self.type
		if t.endswith('key'):	
			if dist(player, self) < 1:
				self.dead = True
				scene.keys[t] = True
		elif self.type == 'player':
			pass
		
		else:
			if dist(self, player) < 1:
				scene.next = ImageScene('death')
				return
			c = self.counter
			if t in ('slime', 'bug'):
				foo = c % 120
				if foo < 60:
					if foo == 0:
						self.dir = random.choice(list('nsew'))
					self.dx = vec[self.dir][0] * .1
					self.dy = vec[self.dir][1] * .1
			elif t in ('ogre', 'wolf'):
				self.dx = sign(player.x - self.x) * .08
				self.dy = sign(player.y - self.y) * .08
			elif t == 'firespit':
				foo = c % 150
				if foo < 40:
					self.spit = True
					if foo == 20:
						s = Sprite('fireball', self.x, self.y)
						ang = random.random() * 6.28
						s.pdx = math.cos(ang) * .2
						s.pdy  = math.sin(ang) * .2
						scene.sprites.append(s)
				else:
					self.spit = False
			elif t == 'fireball':
				if c > 3.5 * 30:
					self.dead = True
			elif t == 'bat':
				foo = c % 90
				if foo == 0:
					ang = random.random() * 6.28
					self.pdx = math.cos(ang) * .1
					self.pdy = math.sin(ang) * .1
			elif t == 'troll':
				if self.var == None:
					self.var = random.random() < .5
				if self.collided:
					self.var = not self.var
				
				self.dx = .1 if self.var else -.1
				
		
		self.counter += 1
		newx = self.x + self.dx + self.pdx
		newy = self.y + self.dy + self.pdy
		
		outside = newx < 0 or newx >= 16 or newy < 0 or newy >= 14
		if self.type == 'fireball' and outside:
			self.dead = True
			return
		
		self.moving = self.dx != 0 or self.dy != 0 or self.pdx != 0 or self.pdy != 0
		self.collided = False
		if not outside and self.moving:
		
			t = scene.tiles[int(newx)][int(newy)]
			passable = t[1]
			if not passable and self.type == 'player' and t[0] in '123':
				if t[0] == '1' and scene.keys.get('redkey', False): passable = True
				if t[0] == '2' and scene.keys.get('greenkey', False): passable = True
				if t[0] == '3' and scene.keys.get('bluekey', False): 
					scene.next = ImageScene('victory')
			if passable:
				if newy > self.y:
					self.dir = 's'
				elif newy < self.y:
					self.dir = 'n'
				elif newx < self.x:
					self.dir = 'w'
				else:
					self.dir = 'e'
				
				self.x = newx
				self.y = newy
				self.moving = True
				
				
			else:
				self.collided = True
		else:
			self.collided = True
		
		
		self.dx = 0
		self.dy = 0
	def render(self, screen, rc):
		if self.spit:
			n = '1'
		elif self.moving and (rc // 5) % 2 == 0:
			n = '1'
		else:
			n = '0'
		img = self.images[self.dir + n]
		screen.blit(img, (int(self.x * 16 - 8), int(self.y * 16 - 8)))
class SC: pass

def load_maps():
	maps = make_grid(10, 6)
	
	for file in os.listdir('maps'):
		path = 'maps' + os.sep + file
		x, y, music = file.split('.')[0].split('-')
		col = int(x) - 1
		row = int(y) - 1
		music = 'overworld' if music == 'o' else 'dungeon'
		
		map = SC()
		map.col = col
		map.row = row
		map.music = music
		tiles = make_grid(16, 14)
		c = open(path, 'rt')
		t = c.read().split('\n')
		c.close()
		
		for y in range(14):
			for x in range(16):
				tiles[x][y] = t[y][x]
		map.tiles = tiles
		maps[col][row] = map
	
	return maps
	
class PlayScene:
	def __init__(self):
		self.next = self
		self.player = Sprite('player', 12.5, 7.5)
		self.tiles = None
		self.sprites = None
		self.maps = load_maps()
		self.swordcounter = -1
		self.swordx = 0
		self.swordy = 0
		self.keys = {}
		self.allowsword = False
		self.current = self.maps[0][5]
		self.music = None
	
	def init_map(self):
		tilestore = load_tiles()
		#tiles.maps
		tiles = make_grid(16, 14)
		sprites = [self.player]
		map = self.maps[self.current.col][self.current.row]
		for y in range(14):
			for x in range(16):
				id = map.tiles[x][y]
				if id in '!@#$%^&*{}?':
					sid = id
					if id == '!': sid = 'slime'
					if id == '@': sid = 'ogre'
					if id == '#': sid = 'bat'
					if id == '$': sid = 'firespit'
					if id == '%': sid = 'troll'
					if id == '^': sid = 'bug'
					if id == '&': sid = 'wolf'
					if id == '*': sid = 'princess'
					if id == '{': sid = 'redkey'
					if id == '}': sid = 'greenkey'
					if id == '?': sid = 'bluekey'
					sprites.append(Sprite(sid, x + .5, y + .5))
					if map.music == 'overworld':
						id = 'L'
					else:
						id = 'I'
				tt = tilestore[id]
				tiles[x][y] = (id, tt.passable, tt.img)
		m = map.music
		if self.music != m:
			pygame.mixer.music.load('music' + os.sep + m + '.ogg')
			pygame.mixer.music.play(-1)
			self.music = m
		self.tiles = tiles
		self.sprites = sprites
		
	def update(self, sp, dx, dy):
		if self.tiles == None:
			self.init_map()
		
		if not sp:
			self.allowsword = True
		
		if self.allowsword  and sp and self.swordcounter < 0:
			self.swordcounter = 6
			self.swordx = (self.player.x + vec[self.player.dir][0])
			self.swordy = (self.player.y + vec[self.player.dir][1])
			
			
		if self.swordcounter < 0:
			self.player.dx = dx * .15
			self.player.dy = dy * .15
		
		ns = []
		for sprite in self.sprites:
			sprite.update(self, self.player)
			if not sprite.dead:
				ns.append(sprite)
		self.sprites = ns
		
		px = self.player.x
		py = self.player.y
		mx = self.current.col
		my = self.current.row
		nx = mx
		ny = my
		if px < .5:
			nx -= 1
			self.player.x = 15.3
		elif px > 15.5:
			nx += 1
			self.player.x = .8
		elif py < .5:
			ny -= 1
			self.player.y = 13.3
		elif py > 13.5:
			ny += 1
			self.player.y = .8
		if nx != mx or ny != my:
			self.tiles = None
			self.current = self.maps[nx][ny]
		
			self.init_map()
		self.swordcounter -= 1
	
	def render(self, screen, rc):
		for y in range(14):
			for x in range(16):
				screen.blit(self.tiles[x][y][2], (x * 16, y * 16))
				
		
		for sprite in self.sprites:
			sprite.render(screen, rc)
		
		if self.swordcounter >= 0:
			x = int(16 * self.swordx) - 8
			y = int(16 * self.swordy) - 8
			w = 16
			h = 16
			if self.player.dir in 'ns':
				x += 6
				w = 4
			else:
				y += 6
				h = 4
			pygame.draw.rect(screen, (255, 255, 255), pygame.Rect(x, y, w, h))
			

class ImageScene:
	def __init__(self, type):
		self.type = type
		self.acceptinput = False
		self.next = self
		
		music = 'title'
		if type == 'death':
			music = 'death'
		elif type == 'story':
			music = None
		if music != None:
			pygame.mixer.music.load('music' + os.sep + music + '.ogg')
			pygame.mixer.music.play(0)
		
	def update(self, sp, dx, dy):
		self.acceptinput = self.acceptinput or ( not sp)
		if self.acceptinput and sp:
			if self.type == 'title':
				self.next = ImageScene('story')
			elif self.type == 'story':
				self.next = PlayScene()
			elif self.type == 'victory' or self.type == 'death':
				self.next = ImageScene("title")
		
	def render(self, screen, rc):
		img = get_image('screens/' + self.type + '.png')
		screen.blit(img, (0, 0))

def main():
	pygame.init()
	rs = pygame.display.set_mode((256 * 3, 224 * 3))
	vs = pygame.Surface((256, 224))
	clk = pygame.time.Clock()
	scene = ImageScene('title')
	rc = 0
	while True:
		
		pr =pygame.key.get_pressed()
		for ev in pygame.event.get():
			if ev.type == pygame.QUIT:
				return
			if ev.type == pygame.KEYDOWN:
				if ev.key == pygame.K_F4 and (pr[pygame.K_LALT] or pr[pygame.K_RALT]):
					return
		
		sp = pr[pygame.K_SPACE] or pr[pygame.K_RETURN]
		dx = -1 if pr[pygame.K_LEFT] else (1 if pr[pygame.K_RIGHT] else 0)
		dy = -1 if pr[pygame.K_UP] else (1 if pr[pygame.K_DOWN] else 0)
		
		rc += 1
		
		scene= scene.next
		scene.update(sp, dx, dy)
		scene.render(vs, rc)
		
		scene = scene.next
		
		pygame.transform.scale(vs, rs.get_size(), rs)
		
		pygame.display.flip()
		
		clk.tick(30)
		
main()