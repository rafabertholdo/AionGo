package data

import "path/filepath"

// Recipe is a recipe_template of recipe_templates.xml: what a craft makes from what.
type Recipe struct {
	ID         int32  `xml:"id,attr"`
	NameID     int32  `xml:"nameid,attr"`
	SkillID    int32  `xml:"skillid,attr"`
	Race       string `xml:"race,attr"` // PC_LIGHT, PC_DARK or ALL
	SkillPoint int32  `xml:"skillpoint,attr"`
	DP         int32  `xml:"dp,attr"`
	Autolearn  int32  `xml:"autolearn,attr"`
	ProductID  int32  `xml:"productid,attr"`
	Quantity   int32  `xml:"quantity,attr"`
	Components []struct {
		ItemID   int32 `xml:"itemid,attr"`
		Quantity int32 `xml:"quantity,attr"`
	} `xml:"component"`
	Combo []struct {
		ItemID int32 `xml:"itemid,attr"`
	} `xml:"comboproduct"`
}

// ComboProduct is what a critical craft makes instead, or 0.
func (r *Recipe) ComboProduct() int32 {
	if len(r.Combo) == 0 {
		return 0
	}
	return r.Combo[0].ItemID
}

// recipeKey is RecipeData's hash of who learns a recipe by skill level.
type recipeKey struct{ skill, point int32 }

func loadRecipes(dir string) (map[int32]*Recipe, error) {
	var file struct {
		List []*Recipe `xml:"recipe_template"`
	}
	if err := loadXML(filepath.Join(dir, "recipe/recipe_templates.xml"), &file); err != nil {
		return nil, err
	}
	recipes := map[int32]*Recipe{}
	for _, r := range file.List {
		recipes[r.ID] = r
	}
	return recipes, nil
}

// AutolearnRecipes is RecipeData.getRecipeIdFor: the recipes a player of the race (PC_LIGHT or PC_DARK) learns on
// its own at that level of the skill.
func (d *Data) AutolearnRecipes(race string, skill, point int32) []*Recipe {
	var found []*Recipe
	for _, r := range d.autolearn[recipeKey{skill, point}] {
		if r.Race == race || r.Race == "ALL" || r.Race == "" {
			found = append(found, r)
		}
	}
	return found
}

func (d *Data) indexRecipes() {
	d.autolearn = map[recipeKey][]*Recipe{}
	for _, r := range d.Recipes {
		if r.Autolearn != 0 {
			k := recipeKey{r.SkillID, r.SkillPoint}
			d.autolearn[k] = append(d.autolearn[k], r)
		}
	}
}
