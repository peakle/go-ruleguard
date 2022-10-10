package gocritic

type pair struct {
    first  int
    second int
}

func fieldSwap(p *pair) {
    tmp := p.first // want `\Qcan use parallel assignment like p.first,p.second=p.second,p.first`
    p.first = p.second
    p.second = tmp
}

func varSwap(x, y int) {
    tmp := x // want `\Qcan use parallel assignment like x,y=y,x`
    x = y
    y = tmp
}

func pointersSwap1(x, y *int) {
    tmp := *x // want `\Qcan use parallel assignment like *x,*y=*y,*x`
    *x = *y
    *y = tmp
}

func pointersSwap2(x, y *int) {
    tmp := *y // want `\Qcan use parallel assignment like *y,*x=*x,*y`
    *y = *x
    *x = tmp
}
