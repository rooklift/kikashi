# Kikashi

Kikashi is a Go / Baduk / Weiqi library for Golang. In particular, it deals with SGF.

# Example

```Golang
package main

import (
    "fmt"

    k "github.com/fohristiwhirl/kikashi"
)

func main() {

    // To create a plain new Tree, you can generally use:
    //
    //      node := k.NewTree(size)
    //
    // But if you want handicap or other stones, one must pass
    // some actual properties and use k.NewNode(nil, props)
    //
    // In this example, we create the ancient Chinese pattern.

    properties := make(map[string][]string)
    properties["AB"] = []string{k.SGFStringFromPoint(3, 3), k.SGFStringFromPoint(15, 15)}   // ["dd", "pp"]
    properties["AW"] = []string{k.SGFStringFromPoint(15, 3), k.SGFStringFromPoint(3, 15)}   // ["pd", "dp"]
    properties["SZ"] = []string{"19"}

    node := k.NewNode(nil, properties)          // nil means this node has no parent (it's the root)

    // We can now make moves.
    // If successful, TryMove() returns the new node.

    node, err := node.TryMove(k.WHITE, 2, 5)
    if err != nil {
        fmt.Printf("%v\n", err)
    }

    // Illegal moves (including suicide and basic ko) will return an error.
    // As a convenience, TryMove() returns the original node in this case.
    // You may still wish to check for errors...

    node, err = node.TryMove(k.WHITE, 2, 5)
    if err != nil {
        fmt.Printf("%v\n", err)                 // Will complain about the occupied point
    }

    // We can go up the tree and create variations.

    node = node.Parent
    node.TryMove(k.WHITE, 2, 6)                 // Create variation 2
    node.TryMove(k.WHITE, 16, 13)               // Create variation 3
    
    for i, child := range node.Children {
        child.SetValue("C", fmt.Sprintf("Comment %d", i)
    }

    // And we can go down those variations if we wish.
    // (Errors ignored here for simplicity.)

    node, _ = node.TryMove(k.WHITE, 13, 16)     // Create variation 4 and go down it
    node, _ = node.TryMove(k.BLACK, 16, 13)     // ...continue going down it
    node, _ = node.TryMove(k.WHITE, 15, 17)     // ...continue going down it

    // We can add properties, EXCEPT board-altering properties...

    val := k.SGFStringFromPoint(15, 17)         // The string "pr" - corresponds to 15,17
    node.AddValue("TR", val)

    // Calling Save() will save the entire tree, regardless of node position.

    node.Save("foo.sgf")

    // We can also load...

    node, _ = k.Load("foo.sgf")
}
```
