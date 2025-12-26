package engine

type Counter struct {
	global int
	perDir map[string]int
}

func NewCounter(start int) *Counter {
	return &Counter{
		global: start,
		perDir: make(map[string]int),
	}
}

func (c *Counter) Next(dir string) int {
	if len(dir) > 0 {
		return c.nextForDir(dir)
	}

	val := c.global
	c.global++
	return val
}

func (c *Counter) nextForDir(dir string) int {
	if _, exists := c.perDir[dir]; !exists {
		c.perDir[dir] = 1
	}
	val := c.perDir[dir]
	c.perDir[dir]++
	return val
}
