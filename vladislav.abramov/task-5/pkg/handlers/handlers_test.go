package handlers_test

import (
	"context"
	"testing"
	"time"

	"github.com/15446-rus75/task-5/pkg/conveyer"
	"github.com/15446-rus75/task-5/pkg/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runConveyerTest(t *testing.T, testFn func(*conveyer.StringConveyer, context.Context, *error), setupFn func(*conveyer.StringConveyer)) {
	t.Helper()

	conv := conveyer.New(5)
	if setupFn != nil {
		setupFn(conv)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var runErr error
	go func() {
		testFn(conv, ctx, &runErr)
		cancel()
	}()

	err := conv.Run(ctx)
	require.NoError(t, err)
	require.NoError(t, runErr)
}

func sendAndAssert(t *testing.T, conv *conveyer.StringConveyer, inName, outName, send, expected string) error {
	t.Helper()

	if err := conv.Send(inName, send); err != nil {
		return err
	}

	res, err := conv.Recv(outName)
	if err != nil {
		return err
	}

	assert.Equal(t, expected, res)
	return nil
}

func TestPrefixDecorator(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterDecorator(handlers.PrefixDecoratorFunc, "in", "out")

		*runErr = sendAndAssert(t, conv, "in", "out", "test", "decorated: test")
	}, nil)
}

func TestDecoratorChain(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterDecorator(handlers.PrefixDecoratorFunc, "in", "mid")
		conv.RegisterDecorator(handlers.PrefixDecoratorFunc, "mid", "out")

		*runErr = sendAndAssert(t, conv, "in", "out", "data", "decorated: data")
	}, nil)
}

func TestDecoratorWithExistingPrefix(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterDecorator(handlers.PrefixDecoratorFunc, "in", "out")

		*runErr = sendAndAssert(t, conv, "in", "out", "decorated: already", "decorated: already")
	}, nil)
}

func TestDecoratorError(t *testing.T) {
	t.Parallel()

	conv := conveyer.New(5)
	conv.RegisterDecorator(handlers.PrefixDecoratorFunc, "in", "out")

	require.NoError(t, conv.Send("in", "text no decorator contains"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := conv.Run(ctx)
	require.ErrorIs(t, err, handlers.ErrNoDecorator)

	res, err := conv.Recv("out")
	require.NoError(t, err)
	assert.Equal(t, "undefined", res)
}

func TestSeparatorRoundRobin(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterSeparator(handlers.SeparatorFunc, "in", []string{"out1", "out2"})

		*runErr = sendAndAssert(t, conv, "in", "out1", "a", "a")
		*runErr = sendAndAssert(t, conv, "in", "out2", "b", "b")
		*runErr = sendAndAssert(t, conv, "in", "out1", "c", "c")
	}, nil)
}

func TestSeparatorThreeOutputs(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterSeparator(handlers.SeparatorFunc, "in", []string{"out1", "out2", "out3"})

		*runErr = sendAndAssert(t, conv, "in", "out1", "1", "1")
		*runErr = sendAndAssert(t, conv, "in", "out2", "2", "2")
		*runErr = sendAndAssert(t, conv, "in", "out3", "3", "3")
	}, nil)
}

func TestSeparatorSingleOutput(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterSeparator(handlers.SeparatorFunc, "in", []string{"out"})

		*runErr = sendAndAssert(t, conv, "in", "out", "single", "single")
		*runErr = sendAndAssert(t, conv, "in", "out", "data", "data")
	}, nil)
}

func TestMultiplexerMultipleInputs(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterMultiplexer(handlers.MultiplexerFunc, []string{"in1", "in2"}, "out")

		*runErr = sendAndAssert(t, conv, "in1", "out", "from1", "from1")
		*runErr = sendAndAssert(t, conv, "in2", "out", "from2", "from2")
	}, nil)
}

func TestMultiplexerFilter(t *testing.T) {
	t.Parallel()

	conv := conveyer.New(5)
	conv.RegisterMultiplexer(handlers.MultiplexerFunc, []string{"in1", "in2"}, "out")

	require.NoError(t, conv.Send("in1", "normal"))
	require.NoError(t, conv.Send("in2", "no multiplexer here"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var runErr error
	go func() {
		runErr = sendAndAssert(t, conv, "in1", "out", "another", "another")
		cancel()
	}()

	err := conv.Run(ctx)
	require.NoError(t, err)
	require.NoError(t, runErr)

	// Проверяем что отфильтрованное сообщение не прошло
	res, err := conv.Recv("out")
	require.NoError(t, err)
	assert.Equal(t, "undefined", res)
}

func TestMultiplexerThreeInputs(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterMultiplexer(handlers.MultiplexerFunc, []string{"in1", "in2", "in3"}, "out")

		*runErr = sendAndAssert(t, conv, "in1", "out", "a", "a")
		*runErr = sendAndAssert(t, conv, "in2", "out", "b", "b")
		*runErr = sendAndAssert(t, conv, "in3", "out", "c", "c")
	}, nil)
}

func TestComplexPipeline(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		// Создаем сложный пайплайн: декор -> разделитель -> мультиплексор
		conv.RegisterDecorator(handlers.PrefixDecoratorFunc, "input", "decorated")
		conv.RegisterSeparator(handlers.SeparatorFunc, "decorated", []string{"split1", "split2"})
		conv.RegisterMultiplexer(handlers.MultiplexerFunc, []string{"split1", "split2"}, "final")

		*runErr = sendAndAssert(t, conv, "input", "final", "test", "decorated: test")
	}, nil)
}

func TestEmptySeparator(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterSeparator(handlers.SeparatorFunc, "in", []string{})
		// Ничего не отправляем - просто проверяем что не падает
	}, nil)
}

func TestEmptyMultiplexer(t *testing.T) {
	t.Parallel()

	runConveyerTest(t, func(conv *conveyer.StringConveyer, _ context.Context, runErr *error) {
		conv.RegisterMultiplexer(handlers.MultiplexerFunc, []string{}, "out")
		// Ничего не отправляем - просто проверяем что не падает
	}, nil)
}
