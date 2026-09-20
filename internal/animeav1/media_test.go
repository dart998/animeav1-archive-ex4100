package animeav1

import "testing"

func TestParseSeriesStateFinalizedWithEpisodeZero(t *testing.T){
	body:=`<h1>Mushoku Tensei II</h1><div>TV Anime • 2023 • Temporada Verano • Finalizado</div><a href="/media/mushoku-tensei-ii/0">0</a><a href="/media/mushoku-tensei-ii/24">24</a>`
	st,err:=parseSeriesState(body,"mushoku-tensei-ii");if err!=nil{t.Fatal(err)};if !st.Finalized||!st.HasEpisodeZero||st.LastEpisode!=24{t.Fatalf("state=%+v",st)}
}

func TestParseSeriesStateAiring(t *testing.T){
	body:=`<h1>Serie</h1><div>TV Anime • En emisión</div><a href="/media/serie/12">12</a>`
	st,err:=parseSeriesState(body,"serie");if err!=nil{t.Fatal(err)};if st.Finalized||st.LastEpisode!=12{t.Fatalf("state=%+v",st)}
}
