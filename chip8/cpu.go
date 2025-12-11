package chip8

import (
	"fmt"
	"math/rand"
)

const (
	MemorySize    = 4096
	RegisterCount = 16
	StackSize     = 16
	ScreenWidth   = 64
	ScreenHeight  = 32
	FontsetSize   = 80
)

// Chip8 represents the entire emulator state
type Chip8 struct {
	// Memory
	memory [MemorySize]uint8

	// Registers
	V  [RegisterCount]uint8 // V0-VF (VF is flag register)
	I  uint16               // Index register
	PC uint16               // Program counter

	// Stack
	stack [StackSize]uint16
	SP    uint8 // Stack pointer

	// Timers (count down at 60Hz)
	delayTimer uint8
	soundTimer uint8

	// Display (64x32 pixels, 1 bit per pixel)
	display [ScreenWidth * ScreenHeight]uint8

	// Keyboard state (16 keys)
	keys [16]bool

	// Flag to indicate if display needs redrawing
	drawFlag bool
}

// Font sprites (0-F), stored in memory at 0x000-0x050
// Each character is 5 bytes (4x5 pixels)
var fontset = [FontsetSize]uint8{
	0xF0, 0x90, 0x90, 0x90, 0xF0, // 0
	0x20, 0x60, 0x20, 0x20, 0x70, // 1
	0xF0, 0x10, 0xF0, 0x80, 0xF0, // 2
	0xF0, 0x10, 0xF0, 0x10, 0xF0, // 3
	0x90, 0x90, 0xF0, 0x10, 0x10, // 4
	0xF0, 0x80, 0xF0, 0x10, 0xF0, // 5
	0xF0, 0x80, 0xF0, 0x90, 0xF0, // 6
	0xF0, 0x10, 0x20, 0x40, 0x40, // 7
	0xF0, 0x90, 0xF0, 0x90, 0xF0, // 8
	0xF0, 0x90, 0xF0, 0x10, 0xF0, // 9
	0xF0, 0x90, 0xF0, 0x90, 0x90, // A
	0xE0, 0x90, 0xE0, 0x90, 0xE0, // B
	0xF0, 0x80, 0x80, 0x80, 0xF0, // C
	0xE0, 0x90, 0x90, 0x90, 0xE0, // D
	0xF0, 0x80, 0xF0, 0x80, 0xF0, // E
	0xF0, 0x80, 0xF0, 0x80, 0x80, // F
}

// New creates and initializes a new Chip8 emulator
func New() *Chip8 {
	c := &Chip8{
		PC: 0x200, // Programs start at 0x200
	}

	// Load fontset into memory (0x000 to 0x050)
	copy(c.memory[:FontsetSize], fontset[:])

	return c
}

// LoadROM loads a ROM into memory starting at 0x200
func (c *Chip8) LoadROM(rom []byte) error {
	if len(rom) > MemorySize-0x200 {
		return fmt.Errorf("ROM too large: %d bytes (max %d)", len(rom), MemorySize-0x200)
	}

	copy(c.memory[0x200:], rom)
	return nil
}

// EmulateCycle executes one CPU cycle
func (c *Chip8) EmulateCycle() {
	// Fetch opcode (2 bytes, big-endian)
	opcode := uint16(c.memory[c.PC])<<8 | uint16(c.memory[c.PC+1])

	// Decode and execute
	c.executeOpcode(opcode)

	// Update timers
	if c.delayTimer > 0 {
		c.delayTimer--
	}
	if c.soundTimer > 0 {
		c.soundTimer--
	}
}

// executeOpcode decodes and executes a single opcode
func (c *Chip8) executeOpcode(opcode uint16) {
	// Extract common opcode parts
	// opcode format: 0xABCD
	nnn := opcode & 0x0FFF             // lowest 12 bits
	n := uint8(opcode & 0x000F)        // lowest 4 bits
	x := uint8((opcode & 0x0F00) >> 8) // lower 4 bits of high byte
	y := uint8((opcode & 0x00F0) >> 4) // upper 4 bits of low byte
	kk := uint8(opcode & 0x00FF)       // lowest 8 bits

	// Decode based on first nibble
	switch opcode & 0xF000 {
	case 0x0000:
		switch opcode {
		case 0x00E0: // 00E0 - CLS: Clear display
			for i := range c.display {
				c.display[i] = 0
			}
			c.drawFlag = true
			c.PC += 2

		case 0x00EE: // 00EE - RET: Return from subroutine
			c.SP--
			c.PC = c.stack[c.SP]
			c.PC += 2

		default:
			fmt.Printf("Unknown opcode: 0x%X\n", opcode)
			c.PC += 2
		}

	case 0x1000: // 1nnn - JP addr: Jump to address nnn
		c.PC = nnn

	case 0x2000: // 2nnn - CALL addr: Call subroutine at nnn
		c.stack[c.SP] = c.PC
		c.SP++
		c.PC = nnn

	case 0x3000: // 3xkk - SE Vx, byte: Skip next instruction if Vx == kk
		if c.V[x] == kk {
			c.PC += 4
		} else {
			c.PC += 2
		}

	case 0x4000: // 4xkk - SNE Vx, byte: Skip next instruction if Vx != kk
		if c.V[x] != kk {
			c.PC += 4
		} else {
			c.PC += 2
		}

	case 0x5000: // 5xy0 - SE Vx, Vy: Skip next instruction if Vx == Vy
		if c.V[x] == c.V[y] {
			c.PC += 4
		} else {
			c.PC += 2
		}

	case 0x6000: // 6xkk - LD Vx, byte: Set Vx = kk
		c.V[x] = kk
		c.PC += 2

	case 0x7000: // 7xkk - ADD Vx, byte: Set Vx = Vx + kk
		c.V[x] += kk
		c.PC += 2

	case 0x8000:
		switch opcode & 0x000F {
		case 0x0000: // 8xy0 - LD Vx, Vy: Set Vx = Vy
			c.V[x] = c.V[y]
			c.PC += 2

		case 0x0001: // 8xy1 - OR Vx, Vy: Set Vx = Vx OR Vy
			c.V[x] |= c.V[y]
			c.PC += 2

		case 0x0002: // 8xy2 - AND Vx, Vy: Set Vx = Vx AND Vy
			c.V[x] &= c.V[y]
			c.PC += 2

		case 0x0003: // 8xy3 - XOR Vx, Vy: Set Vx = Vx XOR Vy
			c.V[x] ^= c.V[y]
			c.PC += 2

		case 0x0004: // 8xy4 - ADD Vx, Vy: Set Vx = Vx + Vy, set VF = carry
			sum := uint16(c.V[x]) + uint16(c.V[y])
			c.V[0xF] = 0
			if sum > 0xFF {
				c.V[0xF] = 1
			}
			c.V[x] = uint8(sum)
			c.PC += 2

		case 0x0005: // 8xy5 - SUB Vx, Vy: Set Vx = Vx - Vy, set VF = NOT borrow
			c.V[0xF] = 0
			if c.V[x] > c.V[y] {
				c.V[0xF] = 1
			}
			c.V[x] -= c.V[y]
			c.PC += 2

		case 0x0006: // 8xy6 - SHR Vx: Set Vx = Vx SHR 1
			c.V[0xF] = c.V[x] & 0x1
			c.V[x] >>= 1
			c.PC += 2

		case 0x0007: // 8xy7 - SUBN Vx, Vy: Set Vx = Vy - Vx, set VF = NOT borrow
			c.V[0xF] = 0
			if c.V[y] > c.V[x] {
				c.V[0xF] = 1
			}
			c.V[x] = c.V[y] - c.V[x]
			c.PC += 2

		case 0x000E: // 8xyE - SHL Vx: Set Vx = Vx SHL 1
			c.V[0xF] = (c.V[x] & 0x80) >> 7
			c.V[x] <<= 1
			c.PC += 2

		default:
			fmt.Printf("Unknown opcode: 0x%X\n", opcode)
			c.PC += 2
		}

	case 0x9000: // 9xy0 - SNE Vx, Vy: Skip next instruction if Vx != Vy
		if c.V[x] != c.V[y] {
			c.PC += 4
		} else {
			c.PC += 2
		}

	case 0xA000: // Annn - LD I, addr: Set I = nnn
		c.I = nnn
		c.PC += 2

	case 0xB000: // Bnnn - JP V0, addr: Jump to location nnn + V0
		c.PC = nnn + uint16(c.V[0])

	case 0xC000: // Cxkk - RND Vx, byte: Set Vx = random byte AND kk
		c.V[x] = uint8(rand.Intn(256)) & kk
		c.PC += 2

	case 0xD000: // Dxyn - DRW Vx, Vy, n: Draw sprite at (Vx, Vy) with height n
		c.drawSprite(x, y, n)
		c.PC += 2

	case 0xE000:
		switch opcode & 0x00FF {
		case 0x009E: // Ex9E - SKP Vx: Skip next instruction if key Vx is pressed
			if c.keys[c.V[x]] {
				c.PC += 4
			} else {
				c.PC += 2
			}

		case 0x00A1: // ExA1 - SKNP Vx: Skip next instruction if key Vx is not pressed
			if !c.keys[c.V[x]] {
				c.PC += 4
			} else {
				c.PC += 2
			}

		default:
			fmt.Printf("Unknown opcode: 0x%X\n", opcode)
			c.PC += 2
		}

	case 0xF000:
		switch opcode & 0x00FF {
		case 0x0007: // Fx07 - LD Vx, DT: Set Vx = delay timer
			c.V[x] = c.delayTimer
			c.PC += 2

		case 0x000A: // Fx0A - LD Vx, K: Wait for key press, store in Vx
			keyPressed := false
			for i := 0; i < 16; i++ {
				if c.keys[i] {
					c.V[x] = uint8(i)
					keyPressed = true
					break
				}
			}
			if keyPressed {
				c.PC += 2
			}
			// If no key pressed, don't increment PC (wait)

		case 0x0015: // Fx15 - LD DT, Vx: Set delay timer = Vx
			c.delayTimer = c.V[x]
			c.PC += 2

		case 0x0018: // Fx18 - LD ST, Vx: Set sound timer = Vx
			c.soundTimer = c.V[x]
			c.PC += 2

		case 0x001E: // Fx1E - ADD I, Vx: Set I = I + Vx
			c.I += uint16(c.V[x])
			c.PC += 2

		case 0x0029: // Fx29 - LD F, Vx: Set I = location of sprite for digit Vx
			c.I = uint16(c.V[x]) * 5 // Each font character is 5 bytes
			c.PC += 2

		case 0x0033: // Fx33 - LD B, Vx: Store BCD representation of Vx in I, I+1, I+2
			c.memory[c.I] = c.V[x] / 100
			c.memory[c.I+1] = (c.V[x] / 10) % 10
			c.memory[c.I+2] = c.V[x] % 10
			c.PC += 2

		case 0x0055: // Fx55 - LD [I], Vx: Store V0 through Vx in memory starting at I
			for i := uint8(0); i <= x; i++ {
				c.memory[c.I+uint16(i)] = c.V[i]
			}
			c.PC += 2

		case 0x0065: // Fx65 - LD Vx, [I]: Read V0 through Vx from memory starting at I
			for i := uint8(0); i <= x; i++ {
				c.V[i] = c.memory[c.I+uint16(i)]
			}
			c.PC += 2

		default:
			fmt.Printf("Unknown opcode: 0x%X\n", opcode)
			c.PC += 2
		}

	default:
		fmt.Printf("Unknown opcode: 0x%X\n", opcode)
		c.PC += 2
	}
}

// drawSprite draws a sprite at coordinates (Vx, Vy) with height n
func (c *Chip8) drawSprite(x, y, height uint8) {
	c.V[0xF] = 0 // Reset collision flag

	xPos := c.V[x] % ScreenWidth
	yPos := c.V[y] % ScreenHeight

	for row := uint8(0); row < height; row++ {
		spriteData := c.memory[c.I+uint16(row)]

		for col := uint8(0); col < 8; col++ {
			// Check if current pixel in sprite is set
			if (spriteData & (0x80 >> col)) != 0 {
				// Calculate screen position
				screenX := (xPos + col) % ScreenWidth
				screenY := (yPos + row) % ScreenHeight
				pixelIndex := screenY*ScreenWidth + screenX

				// Check for collision (pixel already set)
				if c.display[pixelIndex] == 1 {
					c.V[0xF] = 1
				}

				// XOR the pixel
				c.display[pixelIndex] ^= 1
			}
		}
	}

	c.drawFlag = true
}

// SetKey sets the state of a key
func (c *Chip8) SetKey(key uint8, pressed bool) {
	if key < 16 {
		c.keys[key] = pressed
	}
}

// GetDisplay returns the current display state
func (c *Chip8) GetDisplay() [ScreenWidth * ScreenHeight]uint8 {
	return c.display
}

// DrawFlag returns and resets the draw flag
func (c *Chip8) DrawFlag() bool {
	flag := c.drawFlag
	c.drawFlag = false
	return flag
}
