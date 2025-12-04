package pool

import (
	"testing"
)

// TestStruct — тестовая структура с методом Reset().
type TestStruct struct {
	Value  int
	Name   string
	Items  []int
	Active bool
}

func (ts *TestStruct) Reset() {
	ts.Value = 0
	ts.Name = ""
	ts.Items = ts.Items[:0]
	ts.Active = false
}

func TestPool_NewAndGet(t *testing.T) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Items: make([]int, 0, 10),
		}
	})

	obj := p.Get()

	if obj == nil {
		t.Fatal("Expected non-nil object from pool")
	}

	if obj.Items == nil {
		t.Error("Expected Items slice to be initialized")
	}
}

func TestPool_PutCallsReset(t *testing.T) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Items: make([]int, 0, 10),
		}
	})
	obj := p.Get()
	obj.Value = 42
	obj.Name = "test"
	obj.Items = append(obj.Items, 1, 2, 3)
	obj.Active = true

	p.Put(obj)
	obj2 := p.Get()

	if obj2.Value != 0 {
		t.Errorf("Expected Value=0 after reset, got %d", obj2.Value)
	}
	if obj2.Name != "" {
		t.Errorf("Expected Name='' after reset, got %s", obj2.Name)
	}
	if len(obj2.Items) != 0 {
		t.Errorf("Expected Items len=0 after reset, got %d", len(obj2.Items))
	}
	if cap(obj2.Items) != 10 {
		t.Errorf("Expected Items cap=10 (preserved), got %d", cap(obj2.Items))
	}
	if obj2.Active {
		t.Error("Expected Active=false after reset")
	}
}

func TestPool_Reuse(t *testing.T) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Items: make([]int, 0, 10),
		}
	})

	obj1 := p.Get()
	ptr1 := obj1

	obj1.Value = 100
	p.Put(obj1)


	obj2 := p.Get()

	if obj2 != ptr1 {
		t.Log("Note: Got different object (pool behavior is not deterministic)")
	}


	if obj2.Value != 0 {
		t.Errorf("Expected Value=0 in reused object, got %d", obj2.Value)
	}
}

func TestPool_Concurrent(t *testing.T) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Items: make([]int, 0, 10),
		}
	})

	const goroutines = 100
	done := make(chan bool, goroutines)


	for i := 0; i < goroutines; i++ {
		go func(id int) {
			obj := p.Get()
			obj.Value = id
			obj.Name = "concurrent"
			obj.Items = append(obj.Items, id)
			p.Put(obj)
			done <- true
		}(i)
	}


	for i := 0; i < goroutines; i++ {
		<-done
	}


	obj := p.Get()
	if obj.Value != 0 {
		t.Errorf("Expected clean object after concurrent use, Value=%d", obj.Value)
	}
}

// BenchmarkPool_GetPut проверяет производительность пула.
func BenchmarkPool_GetPut(b *testing.B) {
	p := New(func() *TestStruct {
		return &TestStruct{
			Items: make([]int, 0, 100),
		}
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obj := p.Get()
		obj.Value = i
		obj.Items = append(obj.Items, 1, 2, 3, 4, 5)
		p.Put(obj)
	}
}

// BenchmarkWithoutPool сравнивает производительность без использования пула.
func BenchmarkWithoutPool(b *testing.B) {
	for i := 0; i < b.N; i++ {
		obj := &TestStruct{
			Items: make([]int, 0, 100),
		}
		obj.Value = i
		obj.Items = append(obj.Items, 1, 2, 3, 4, 5)
	}
}
