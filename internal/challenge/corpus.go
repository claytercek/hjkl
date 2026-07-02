package challenge

// CorpusEntry is a multi-line buffer suitable for use in challenge generation.
type CorpusEntry struct {
	Lines []string
}

// curatedCorpus is the curated set of buffer texts used by the generator.
// It includes English prose, pangrams, and simple code-like lines.
var curatedCorpus = []CorpusEntry{
	{Lines: []string{"the quick brown fox jumps over the lazy dog"}},
	{Lines: []string{"pack my box with five dozen liquor jugs"}},
	{Lines: []string{"how vexingly quick daft zebras jump"}},
	{Lines: []string{"the five boxing wizards jump quickly"}},
	{Lines: []string{"sphinx of black quartz judge my vow"}},
	{Lines: []string{"two driven jocks help fax my big quiz"}},
	{Lines: []string{"bright vixens jump dozy fowl quack"}},
	{Lines: []string{"a quick movement of the enemy will jeopardize six gunboats"}},
	{Lines: []string{"all questions asked by five watched experts amaze the judge"}},
	{Lines: []string{"back in my quaint garden jaunty zinnias bloom"}},
	{Lines: []string{"cozy lummox gives smart squid who asks for job pen"}},
	{Lines: []string{"the jay pig fox zebra and my wolves quack"}},
	{Lines: []string{"how razorback jumping frogs can level six piqued gymnasts"}},
	{Lines: []string{"sixty zippers were quickly picked from the woven jute bag"}},
	{Lines: []string{"crazy fredrick bought many very exquisite opal jewels"}},
	{Lines: []string{"we promptly judged antique ivory buckles for the next prize"}},
	{Lines: []string{"a mad boxer shot a quick gloved jab to the jaw of his dizzy opponent"}},
	{Lines: []string{"jaded zombies acted quaintly but kept driving their oxen forward"}},
	{Lines: []string{"the public was amazed to view the quickness and dexterity of the juggler"}},
	{Lines: []string{"the boy was sent to the store for a bag of flour and a jug of milk"}},
	// Multi-line prose
	{Lines: []string{
		"the programmer opened the editor and began to type",
		"she moved the cursor with quick precise motions",
		"every keystroke brought the solution closer",
	}},
	{Lines: []string{
		"the sun rose over the mountains casting long shadows",
		"a gentle breeze stirred the leaves on the ancient oak",
		"birds began their morning songs in the quiet valley",
		"the river flowed steadily carrying whispers downstream",
	}},
	{Lines: []string{
		"the library stood quiet in the afternoon light",
		"rows of books lined the walls from floor to ceiling",
		"dust motes danced in the golden sunbeams",
	}},
	// Code-like lines
	{Lines: []string{
		"func main() {",
		"  fmt.Println(\"hello world\")",
		"  return",
		"}",
	}},
	{Lines: []string{
		"if err != nil {",
		"  return fmt.Errorf(\"failed: %w\", err)",
		"}",
	}},
	{Lines: []string{
		"for i := 0; i < len(items); i++ {",
		"  process(items[i])",
		"}",
	}},
	{Lines: []string{
		"type Config struct {",
		"  Name  string",
		"  Value int",
		"  Flags []string",
		"}",
	}},
}

// curatedLines is a flat list of individual lines extracted from the corpus.
// Used by templates that need a single line.
var curatedLines []string

func init() {
	seen := map[string]bool{}
	for _, entry := range curatedCorpus {
		for _, line := range entry.Lines {
			if !seen[line] {
				seen[line] = true
				curatedLines = append(curatedLines, line)
			}
		}
	}
}
