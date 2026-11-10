/*
|--------------------------------------------------------------------------
| Article Chunks
|--------------------------------------------------------------------------
|
| A published version is cut into the same chunks wherever it is read:
| by the keyword ranking at search time and by the stored index the
| vectors belong to. A chunk's id derives from its version and place,
| so both see one chunk under one id and their rankings fuse. A chunk
| is embedded from its title, headings and text, and the hash of that
| input names its vector: unchanged text is never embedded twice.
|
*/

package knowledge

import (
	"strconv"
	"strings"

	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
ChunkArticle cuts an article into chunks, ids derived from its version
when it has one.
*/
func ChunkArticle(a Article) []contracts.KnowledgeChunk {
	chunks := cutArticle(a)
	if a.VersionID == (ids.UUID{}) {
		return chunks
	}
	for i := range chunks {
		chunks[i].ID = ChunkID(a.VersionID, i)
	}
	return chunks
}

/*
ChunkID is the id of a version's chunk at the ordinal.
*/
func ChunkID(version ids.UUID, ordinal int) ids.UUID {
	return ids.Derive(version, "chunk:"+strconv.Itoa(ordinal))
}

/*
EmbedInput is the text a chunk is embedded from: its title, the
headings below the title, and its text.
*/
func EmbedInput(title string, headingPath []string, text string) string {
	headings := headingPath
	if len(headings) > 0 && headings[0] == title {
		headings = headings[1:]
	}
	return title + "\n" + strings.Join(headings, " / ") + "\n\n" + text
}

/*
EmbedHash names an embed input's vector.
*/
func EmbedHash(input string) string {
	return hashing.SHA256Hex([]byte(input))
}
