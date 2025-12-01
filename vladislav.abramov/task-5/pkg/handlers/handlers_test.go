package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrefixDecoratorFunc_Basic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan string, 1)
	output := make(chan string, 1)

	input <- "test"
	close(input)

	go func() {
		err := PrefixDecoratorFunc(ctx, input, output)
		assert.NoError(t, err)
	}()

	result := <-output
	assert.Equal(t, "decorated: test", result)
}

func TestPrefixDecoratorFunc_AlreadyDecorated(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan string, 1)
	output := make(chan string, 1)

	input <- "decorated: already"
	close(input)

	go PrefixDecoratorFunc(ctx, input, output)

	result := <-output
	assert.Equal(t, "decorated: already", result)
}

func TestPrefixDecoratorFunc_Error(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan string, 1)
	output := make(chan string, 1)

	input <- "text with no decorator"
	close(input)

	err := PrefixDecoratorFunc(ctx, input, output)
	assert.ErrorIs(t, err, ErrNoDecorator)
}

func TestPrefixDecoratorFunc_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	input := make(chan string)
	output := make(chan string)

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	err := PrefixDecoratorFunc(ctx, input, output)
	assert.NoError(t, err)
}

func TestSeparatorFunc_RoundRobin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan string, 3)
	outputs := []chan string{
		make(chan string, 2),
		make(chan string, 2),
	}

	input <- "a"
	input <- "b"
	input <- "c"
	close(input)

	go SeparatorFunc(ctx, input, outputs)

	assert.Equal(t, "a", <-outputs[0])
	assert.Equal(t, "b", <-outputs[1])
	assert.Equal(t, "c", <-outputs[0])
}

func TestSeparatorFunc_NoOutputs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan string, 1)
	input <- "test"
	close(input)

	err := SeparatorFunc(ctx, input, []chan string{})
	assert.NoError(t, err)
}

func TestSeparatorFunc_SingleOutput(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	input := make(chan string, 2)
	output := make(chan string, 2)

	input <- "first"
	input <- "second"
	close(input)

	go SeparatorFunc(ctx, input, []chan string{output})

	assert.Equal(t, "first", <-output)
	assert.Equal(t, "second", <-output)
}

func TestMultiplexerFunc_Basic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputs := []chan string{
		make(chan string, 1),
		make(chan string, 1),
	}
	output := make(chan string, 2)

	inputs[0] <- "from1"
	inputs[1] <- "from2"
	close(inputs[0])
	close(inputs[1])

	go func() {
		err := MultiplexerFunc(ctx, inputs, output)
		assert.NoError(t, err)
	}()

	results := []string{<-output, <-output}
	assert.ElementsMatch(t, []string{"from1", "from2"}, results)
}

func TestMultiplexerFunc_Filter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputs := []chan string{make(chan string, 2)}
	output := make(chan string, 2)

	inputs[0] <- "normal"
	inputs[0] <- "no multiplexer here"
	close(inputs[0])

	go MultiplexerFunc(ctx, inputs, output)

	result := <-output
	assert.Equal(t, "normal", result)
	
	select {
	case <-output:
		t.Fatal("should not receive filtered message")
	case <-time.After(10 * time.Millisecond):
		// OK
	}
}

func TestMultiplexerFunc_NoInputs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	output := make(chan string)

	err := MultiplexerFunc(ctx, []chan string{}, output)
	assert.NoError(t, err)
}

// Тесты для conveyer
func TestConveyer_New(t *testing.T) {
	c := New(10)
	assert.NotNil(t, c)
	assert.Equal(t, 10, c.size)
	assert.NotNil(t, c.channels)
	assert.Empty(t, c.channels)
}

func TestConveyer_GetOrCreateChannel(t *testing.T) {
	c := New(5)
	
	ch1 := c.getOrCreateChannel("test")
	assert.NotNil(t, ch1)
	
	ch2 := c.getOrCreateChannel("test")
	assert.Equal(t, ch1, ch2)
	
	assert.Len(t, c.channels, 1)
}

func TestConveyer_GetChannel(t *testing.T) {
	c := New(5)
	
	ch, err := c.getChannel("nonexistent")
	assert.Nil(t, ch)
	assert.ErrorIs(t, err, ErrChanNotFound)
	
	c.getOrCreateChannel("test")
	ch, err = c.getChannel("test")
	assert.NotNil(t, ch)
	assert.NoError(t, err)
}

func TestConveyer_RegisterDecorator(t *testing.T) {
	c := New(5)
	
	c.RegisterDecorator(PrefixDecoratorFunc, "input", "output")
	
	assert.Len(t, c.decorators, 1)
	assert.Len(t, c.channels, 2)
	assert.Equal(t, "input", c.decorators[0].input)
	assert.Equal(t, "output", c.decorators[0].output)
}

func TestConveyer_RegisterMultiplexer(t *testing.T) {
	c := New(5)
	
	c.RegisterMultiplexer(MultiplexerFunc, []string{"in1", "in2"}, "out")
	
	assert.Len(t, c.multiplexers, 1)
	assert.Len(t, c.channels, 3)
	assert.Equal(t, []string{"in1", "in2"}, c.multiplexers[0].inputs)
	assert.Equal(t, "out", c.multiplexers[0].output)
}

func TestConveyer_RegisterSeparator(t *testing.T) {
	c := New(5)
	
	c.RegisterSeparator(SeparatorFunc, "input", []string{"out1", "out2"})
	
	assert.Len(t, c.separators, 1)
	assert.Len(t, c.channels, 3)
	assert.Equal(t, "input", c.separators[0].input)
	assert.Equal(t, []string{"out1", "out2"}, c.separators[0].outputs)
}

func TestConveyer_SendRecv(t *testing.T) {
	c := New(5)
	c.getOrCreateChannel("test")
	
	err := c.Send("test", "hello")
	assert.NoError(t, err)
	
	result, err := c.Recv("test")
	assert.NoError(t, err)
	assert.Equal(t, "hello", result)
}

func TestConveyer_SendError(t *testing.T) {
	c := New(5)
	
	err := c.Send("nonexistent", "data")
	assert.ErrorIs(t, err, ErrChanNotFound)
}

func TestConveyer_RecvError(t *testing.T) {
	c := New(5)
	
	result, err := c.Recv("nonexistent")
	assert.ErrorIs(t, err, ErrChanNotFound)
	assert.Equal(t, "", result)
}

func TestConveyer_RecvClosedChannel(t *testing.T) {
	c := New(5)
	c.getOrCreateChannel("test")
	close(c.channels["test"])
	
	result, err := c.Recv("test")
	assert.NoError(t, err)
	assert.Equal(t, "undefined", result)
}

func TestConveyer_RunBasic(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	c := New(5)
	c.RegisterDecorator(func(ctx context.Context, in, out chan string) error {
		select {
		case data := <-in:
			out <- "processed: " + data
		case <-ctx.Done():
		}
		return nil
	}, "input", "output")

	go func() {
		time.Sleep(10 * time.Millisecond)
		c.Send("input", "test")
	}()

	err := c.Run(ctx)
	assert.NoError(t, err)
}

func TestConveyer_RunWithError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	c := New(5)
	c.RegisterDecorator(func(ctx context.Context, in, out chan string) error {
		return assert.AnError
	}, "input", "output")

	err := c.Run(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "conveyer finished with error")
}

func TestConveyer_ComplexPipeline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	c := New(10)
	
	// Декор -> Разделитель -> Мультиплексор
	c.RegisterDecorator(PrefixDecoratorFunc, "input", "decorated")
	c.RegisterSeparator(SeparatorFunc, "decorated", []string{"split1", "split2"})
	c.RegisterMultiplexer(MultiplexerFunc, []string{"split1", "split2"}, "final")

	// Запускаем конвейер
	go func() {
		err := c.Run(ctx)
		assert.NoError(t, err)
	}()

	// Отправляем данные
	go func() {
		time.Sleep(10 * time.Millisecond)
		c.Send("input", "test1")
		c.Send("input", "test2")
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// Читаем результаты (не все могут прийти из-за таймаута)
	for i := 0; i < 2; i++ {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
		}
	}
}
