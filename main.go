package main

import (
	"bufio"
	"fmt"
	"slices"

	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/TwiN/go-color"
)

/*
idjfdjifji
Rychlejší a jednodušší optimalizovaná minimalistická implementace Ondrova algoritmu.
Na začátku vygeneruje všechy možné kombinace a při každém guessu je projde
a vyřadí ty, které neodpovídají dosud získaným hodnocením.
Pukud je počet možných kombinací větší než FULL_SEARCH_TRESHOLD, vybere jako svůj
guess jednu náhodně. Pokud je menší než FULL_SEARCH_TRESHOLD, pro všechny možné kombinace spočítá,
kolik možností za seznamu eliminují, pokud by je guessnul, a vybere tu, která počet kombinací zmenší
nejvíce. Pokud je navíc kombinací méně než LOOP_OVER_ALL_TRESHOLD, kontroluje i kombinace, které již
byly vyřazeny, zda počet kombinací nezmenší ještě více. Viz Ondrův komentář.
Program vyžaduje minimálně jeden parametr, který určuje, zda hraje hráč (player), počítač (computer)
nebo počítač s automatickým hodnocením (auto). Hodnocení guessu je ve formě např. xx.., kde x je pin správné barvy
na správném místě a . je pin správné barvy na špatném místě. Při hodnocení mají x piny přednost
a piny zabírají místa . pinům, tj. guess AABD a řešení ADBC by dostalo hodnocení "xx.". Druhé A nedostane
., protože na jeho místě je již první A.
*/

const (
  SIZE = 6
  NUM_COLORS = 6
  FULL_SEARCH_TRESHOLD = 1140
  LOOP_OVER_ALL_TRESHOLD = 70
  VERBOSE = false
)
var (
  THREADS = 8
  CHARS = strings.Split("ABCDEF", "")
  COLORS = []string{
    color.Red,
    color.Green,
    color.Blue,
    color.Yellow,
    color.Purple,
    color.Cyan,
  }
)

type Comb struct {
  val [SIZE]int
  cnt [NUM_COLORS]int
}

type ThreadRes struct {
  comb Comb
  maxrem int
}

type GuessIter struct {
  guess Comb
  right, swap int
}

func gen_comb(val [SIZE]int) Comb {
  res := Comb{val, [NUM_COLORS]int{}}
  for _, e := range val { res.cnt[e] ++ }
  return res
}

func split_slice[T any](s []T, n int) [][]T {
  divided := [][]T{}
  chunkSize := (len(s) + n - 1) / n
  for i := 0; i < len(s); i += chunkSize {
    end := i + chunkSize
    if end > len(s) {
      end = len(s)
    }
    divided = append(divided, s[i:end])
  }
  return divided
}

func grade(guess, trgt Comb) (pos, col int) {
  for i := range NUM_COLORS {
    col += min(guess.cnt[i], trgt.cnt[i])
  }
  for i := range SIZE {
    if guess.val[i] == trgt.val[i] {
      pos ++
      col --
    }
  }
  return pos, col
}

func find_best_guess(loopover_combs, loc_combs []Comb, collect chan ThreadRes) {
  best_rem := 0
  best_guess := Comb{}
  for _, a := range loopover_combs {
    rem := 0
    for bi := range len(loc_combs)-1 {
      for ci := range len(loc_combs)-1-bi {
        b,c := loc_combs[bi],loc_combs[ci+bi+1]
        ab_col, ac_col := 0, 0
        pos_score := 0
        for i := range NUM_COLORS {
          ab_col += min(a.cnt[i], b.cnt[i])
          ac_col += min(a.cnt[i], c.cnt[i])
        }
        for i := range SIZE {
          if a.val[i] == b.val[i] {
            pos_score ++
            ab_col --
          }
          if a.val[i] == c.val[i] {
            pos_score --
            ac_col --
          }
        }
        if pos_score == 0 && ab_col == ac_col { continue }
        rem ++
      }
    }
    if rem > best_rem {
      best_rem = rem
      best_guess = a
    }
  }
  collect <- ThreadRes{
    comb: best_guess,
    maxrem: best_rem,
  }
}

func find_best_guess_parallel(loopover, loc_combs []Comb) Comb {
  split := split_slice(loc_combs, THREADS)
  if loopover != nil {
    split = split_slice(loopover, THREADS)
  }
  collect := make(chan ThreadRes)
  if len(split) < THREADS {
    if loopover != nil {
      go find_best_guess(loopover, loc_combs, collect)
    } else {
      go find_best_guess(loc_combs, loc_combs, collect)
    }
    res := <-collect
    return res.comb
  }
  for _, chunk := range split {
    go find_best_guess(chunk, loc_combs, collect)
  }
  best_guesses := []ThreadRes{}
  for len(best_guesses) < THREADS {
    best_guesses = append(best_guesses, <-collect)
  }
  res, maxrem := Comb{}, 0
  for _, tres := range best_guesses {
    if tres.maxrem > maxrem {
      maxrem = tres.maxrem
      res = tres.comb
    }
  }
  return res
}

func gen_all_combs() []Comb {
  loc_combs := []Comb{}
  cur := [SIZE]int{}
  max := [SIZE]int{}
  for i := range SIZE {
    max[i] = NUM_COLORS-1
  }
  loc_combs = append(loc_combs, gen_comb(cur))
  for cur != max {
    for dig := range SIZE {
      cur[dig] ++
      if cur[dig] == NUM_COLORS {
        cur[dig] = 0
      } else { break }
    }
    loc_combs = append(loc_combs, gen_comb(cur))
  }
  return loc_combs
}

func processHistory(history []GuessIter, comb Comb) bool {
  for _, h := range history {
    new_r, new_s := grade(h.guess, comb)
    if new_r != h.right || new_s != h.swap { return false }
  }
  return true
}

func getGrade(guess Comb, trgt ...Comb) (pos, col int) {
  for _, c := range guess.val {
    fmt.Print(color.Ize(COLORS[c], CHARS[c]))
  }
  if len(trgt) > 0 {
    r,s := grade(guess, trgt[0])
    fmt.Printf(
      " %v%v\n",
      strings.Repeat("x",r),
      strings.Repeat(".",s),
    )
    return r,s
  }
  fmt.Print(" ")
  in := bufio.NewReader(os.Stdin)
  line, _ := in.ReadString('\n')
  for _, c := range line {
    if c == 'x' { pos ++ }
    if c == '.' { col ++ }
  }
  return pos, col
}

func computer_guess(auto bool) {
  all_combs := gen_all_combs()
  combs := gen_all_combs()
  history := []GuessIter{}
  attempt := 1
  solution_raw := [SIZE]int{}
  for i := range solution_raw {
    solution_raw[i] = rand.Intn(NUM_COLORS)
  }
  solution := gen_comb(solution_raw)
  for {
    if len(combs) == 0 {
      fmt.Print("you entered bad rating somewhere\n")
      break
    }
    guess := combs[rand.Intn(len(combs))]
    if len(combs) == 1 {
      if VERBOSE { fmt.Printf("one comb remaining\n") }
      guess = combs[0]
    } else if len(combs) < LOOP_OVER_ALL_TRESHOLD {
      if VERBOSE { fmt.Printf("loop over all %d\n", len(combs)) }
      guess = find_best_guess_parallel(all_combs, combs)
    } else if len(combs) < FULL_SEARCH_TRESHOLD {
      if VERBOSE { fmt.Printf("full search %d\n", len(combs)) }
      guess = find_best_guess_parallel(nil, combs)
    } else if VERBOSE {
      fmt.Printf("picking random comb %d\n", len(combs))
    }
    fmt.Printf("%d. ", attempt)
    right, swapped := 0,0
    if auto {
      right, swapped = getGrade(guess, solution)
    } else {
      right, swapped = getGrade(guess)
    }
    history = append(history, GuessIter{
      guess: guess,
      right: right,
      swap: swapped,
    })
    if right == SIZE {
      fmt.Printf("solved\n")
      break
    }
    attempt ++
    new_combs := []Comb{}
    for _, c := range combs {
      if !processHistory(history, c) { continue }
      new_combs = append(new_combs, c)
    }
    combs = new_combs
  }
}

func player_input() Comb {
  in := bufio.NewReader(os.Stdin)
  line, _ := in.ReadString('\n')
  resval := [SIZE]int{}
  for i, c := range line {
    idx := slices.Index(CHARS, string(c))
    if idx == -1 { continue }
    resval[i] = idx
  }
  return gen_comb(resval)
}

func player_guess() {
  solution_raw := [SIZE]int{}
  for i := range solution_raw {
    solution_raw[i] = rand.Intn(NUM_COLORS)
  }
  solution := gen_comb(solution_raw)
  attempt := 1
  for {
    guess := player_input()
    right, swap := grade(guess, solution)
    fmt.Printf("%2d. ", attempt)
    for _, c := range guess.val {
      fmt.Print(color.Ize(COLORS[c], CHARS[c]))
    }
    rating := strings.Repeat("x", right) +
              strings.Repeat(".", swap)
    fmt.Printf(" %v%v", rating, strings.Repeat(" ", 7 - len(rating)))
    if right == SIZE {
      fmt.Print("\nsolved\n")
      break
    }
    attempt ++
  }
}

func main() {
  if len(os.Args) < 2 {
    fmt.Print("please specify who's guessing in the first parameter\n")
  }
  if len(os.Args) > 2 {
    THREADS, _ = strconv.Atoi(os.Args[1])
  }
  if os.Args[1] == "c" || os.Args[1] == "computer" {
    computer_guess(false)
  }
  if os.Args[1] == "a" || os.Args[1] == "auto" {
    computer_guess(true)
  }
  if os.Args[1] == "p" || os.Args[1] == "player" {
    player_guess()
  }
}
