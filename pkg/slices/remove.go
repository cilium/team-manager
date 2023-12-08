package slices

func Remove[T comparable](slice []T, elementToRemove T) []T {
	for i, element := range slice {
		if element == elementToRemove {
			slice[i] = slice[len(slice)-1]
			return slice[:len(slice)-1]
		}
	}
	return slice
}
