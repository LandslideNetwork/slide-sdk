package state

import (
	"encoding/hex"
	"testing"
)

func TestDecodeBlock(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr bool
	}{
		{
			name:    "failed - BlockIDFlag: 0",
			args:    "0A81030A04080B10011211746573742D636861696E2D5538746537351802220C088ADE99B20610E08DE281012A480A20B36A211648CC884E0756EBCD822FC9DBDBF2093ED7AD2657659E8B5699C2890F122408011220C0F4664FF5641CE97A9D417DB130F44F391772F908092A76112944181872850A322096C552EA4D7C23C64E0938EE43CCEDBA2E1453AA96B7B6B72FD3BD511F2E515F3A20A7A02E7CBB627D642622B06B4B51A227CC806382CF10333C84267C014DE307D04220F886C461EEC1C571508C865E60DED596B2560F7461501E9334BA135597AD9C854A20F886C461EEC1C571508C865E60DED596B2560F7461501E9334BA135597AD9C855220048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F5A0800000000000000006220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A20E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8557214000000000000000000000000000000000000000012170A1571756572795F6B65793D71756572795F76616C75651A0022221A021200220D1A0B088092B8C398FEFFFFFF01220D1A0B088092B8C398FEFFFFFF01",
			wantErr: true,
		},
		{
			name:    "failed - expected ValidatorAddress size to be 20 bytes, got 0 bytes", // BlockIDFlag: types.BlockIDFlagNil,
			args:    "0A81030A04080B10011211746573742D636861696E2D5538746537351802220C08CCF999B20610B08798AA012A480A20B36A211648CC884E0756EBCD822FC9DBDBF2093ED7AD2657659E8B5699C2890F122408011220C0F4664FF5641CE97A9D417DB130F44F391772F908092A76112944181872850A3220B4E6C5385E72E9433F51492791A599620DB572CDC4AF3787D522784DBF92F8203A20A7A02E7CBB627D642622B06B4B51A227CC806382CF10333C84267C014DE307D04220F886C461EEC1C571508C865E60DED596B2560F7461501E9334BA135597AD9C854A20F886C461EEC1C571508C865E60DED596B2560F7461501E9334BA135597AD9C855220048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F5A0800000000000000006220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A20E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8557214000000000000000000000000000000000000000012170A1571756572795F6B65793D71756572795F76616C75651A0022261A021200220F08031A0B088092B8C398FEFFFFFF01220F08031A0B088092B8C398FEFFFFFF01",
			wantErr: true,
		},
		{
			name:    "success - BlockIDFlag: 1", // BlockIDFlag: types.BlockIDFlagAbsent, fail parse block after it
			args:    "0A81030A04080B10011211746573742D636861696E2D5538746537351802220C08F9829CB20610E0F4DD93012A480A20B36A211648CC884E0756EBCD822FC9DBDBF2093ED7AD2657659E8B5699C2890F122408011220C0F4664FF5641CE97A9D417DB130F44F391772F908092A76112944181872850A3220D3ABD0D1E4DFF61D04E00A34B16528A4A7B20EC0CBA19210A44945D7324009B73A20A7A02E7CBB627D642622B06B4B51A227CC806382CF10333C84267C014DE307D04220F886C461EEC1C571508C865E60DED596B2560F7461501E9334BA135597AD9C854A20F886C461EEC1C571508C865E60DED596B2560F7461501E9334BA135597AD9C855220048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F5A0800000000000000006220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A20E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8557214000000000000000000000000000000000000000012170A1571756572795F6B65793D71756572795F76616C75651A0022261A021200220F08011A0B088092B8C398FEFFFFFF01220F08011A0B088092B8C398FEFFFFFF01",
			wantErr: false,
		},
		{
			name:    "success - BlockIDFlag: 3", // types.BlockIDFlagNil, stuck after block
			args:    "0A81030A04080B10011211746573742D636861696E2D5538746537351802220C08929A9CB20610C88ECBC1012A480A20B36A211648CC884E0756EBCD822FC9DBDBF2093ED7AD2657659E8B5699C2890F122408011220C0F4664FF5641CE97A9D417DB130F44F391772F908092A76112944181872850A32205CFC76BB770AE729F8FCFF1BBC9BD6BFA9DD917E1714B3A5973B1F7B1B3057633A20A7A02E7CBB627D642622B06B4B51A227CC806382CF10333C84267C014DE307D04220F886C461EEC1C571508C865E60DED596B2560F7461501E9334BA135597AD9C854A20F886C461EEC1C571508C865E60DED596B2560F7461501E9334BA135597AD9C855220048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F5A0800000000000000006220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A20E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8557214000000000000000000000000000000000000000012170A1571756572795F6B65793D71756572795F76616C75651A00225A1A02120022290803121427A081B935F841C87EB400A37BFA0E7DDC6897E71A0C08929A9CB20610F881CAC101220100222908031214E64417981307C18EB094B5AFA17DEE33165024501A0C08929A9CB20610E089CAC101220100",
			wantErr: false,
		},
		{
			name:    "Parse block error after bootstrapping", //  err="proto: Data: wiretype end group for non-group"
			args:    "0A0E0A04080B10011206302E33382E361211746573742D636861696E2D553874653735180122480A20B36A211648CC884E0756EBCD822FC9DBDBF2093ED7AD2657659E8B5699C2890F122408011220C0F4664FF5641CE97A9D417DB130F44F391772F908092A76112944181872850A2A0C08FEFD8AA006108293A2FE0132D2010A470A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A20F6FFFFFFFFFFFFFFFF010A3E0A14E64417981307C18EB094B5AFA17DEE331650245012220A2097D5FDF9F8E4781CC37CF19B50CEC0308444EAE24DEFCCCDE7E964E4E628B5E9180A200A12470A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A20F6FFFFFFFFFFFFFFFF013ABA010A3C0A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A0A3C0A14E64417981307C18EB094B5AFA17DEE331650245012220A2097D5FDF9F8E4781CC37CF19B50CEC0308444EAE24DEFCCCDE7E964E4E628B5E9180A123C0A14E64417981307C18EB094B5AFA17DEE331650245012220A2097D5FDF9F8E4781CC37CF19B50CEC0308444EAE24DEFCCCDE7E964E4E628B5E9180A42D2010A470A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A20F6FFFFFFFFFFFFFFFF010A3E0A14E64417981307C18EB094B5AFA17DEE331650245012220A2097D5FDF9F8E4781CC37CF19B50CEC0308444EAE24DEFCCCDE7E964E4E628B5E9180A200A12470A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A20F6FFFFFFFFFFFFFFFF01480152330A10088080C00A10FFFFFFFFFFFFFFFFFF01120E08A08D0612040880C60A188080401A0B0A09736563703235366B3122002A0058016220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A0800000000000000007001",
			wantErr: true,
		},
		{
			name:    "Valid block, round 0",
			args:    "0A81030A04080B10011211746573742D636861696E2D5538746537351802220C08A792ACB20610F8A891A6012A480A2006800F2931FA4CDBE470D389C8FAAB5A50BB4FD65F23C1EBB5C69FF44512B1E0122408011220AD47C5976CD4BAC006E22A75A1C0B731FC084E3EAAEE9865934E0D91264159BC322079F5B35B7DDD0C79ECB3C3759EA3FE495D818E63FD2C4C008DCE82B5FE5F8CCD3A20A7A02E7CBB627D642622B06B4B51A227CC806382CF10333C84267C014DE307D04220D4E61E10FD72E0540B7991912A4320B5D48283A3854C62E69DE6DD3B4BA2C9FE4A20D4E61E10FD72E0540B7991912A4320B5D48283A3854C62E69DE6DD3B4BA2C9FE5220048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F5A0800000000000000006220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A20E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8557214000000000000000000000000000000000000000012170A1571756572795F6B65793D71756572795F76616C75651A00227708011A480A2006800F2931FA4CDBE470D389C8FAAB5A50BB4FD65F23C1EBB5C69FF44512B1E0122408011220AD47C5976CD4BAC006E22A75A1C0B731FC084E3EAAEE9865934E0D91264159BC222908031214D4CAE735FFC8559F79A26DB8B75E39395F97C2AE1A0C08A792ACB20610F8DEFFA501220100",
			wantErr: false,
		},
		{
			name:    "init chain block",
			args:    "0ABB020A04080B10011211746573742D636861696E2D5538746537351801220C08FEFD8AA006108293A2FE012A0212003220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8553A20E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8554220D4E61E10FD72E0540B7991912A4320B5D48283A3854C62E69DE6DD3B4BA2C9FE4A20D4E61E10FD72E0540B7991912A4320B5D48283A3854C62E69DE6DD3B4BA2C9FE5220048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F5A0800000000000000006220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A20E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8557214000000000000000000000000000000000000000012001A0022041A021200",
			wantErr: false,
		},
		{
			name:    "panic: runtime error: hash of unhashable type bytes.HexBytes",
			args:    "0A81030A04080B10011211746573742D636861696E2D5538746537351802220C08E988ADB20610C0AF84A7012A480A20E0B7A0713CCE588BA87309C36B356F91395B16BA11AF7267E51AC3857EF4CDFD122408011220C095A6C8EFBECDAC21C7F8CA670A220E29C7C7791D1038ABEE33840C789CBF2C3220312627D661CDA3DCA0F2DDDB478E2085DCB565D89A240C52DFC263B7CEF678193A20A7A02E7CBB627D642622B06B4B51A227CC806382CF10333C84267C014DE307D0422082B1B3AC05FFA5D6F9884262659F5F98E8E9948EB25EFE8C067DCE1D25A084494A2082B1B3AC05FFA5D6F9884262659F5F98E8E9948EB25EFE8C067DCE1D25A084495220048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F5A0800000000000000006220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A20E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8557214000000000000000000000000000000000000000012170A1571756572795F6B65793D71756572795F76616C75651A00227708011A480A20E0B7A0713CCE588BA87309C36B356F91395B16BA11AF7267E51AC3857EF4CDFD122408011220C095A6C8EFBECDAC21C7F8CA670A220E29C7C7791D1038ABEE33840C789CBF2C22290803121427A081B935F841C87EB400A37BFA0E7DDC6897E71A0C08E988ADB20610F8EAF8A601220100",
			wantErr: false,
		},
		{
			name:    "panic: runtime error: hash of unhashable type bytes.HexBytes",
			args:    "0A81030A04080B10011211746573742D636861696E2D5538746537351802220C08B8A0ADB20610A8AFD9D1012A480A20E0B7A0713CCE588BA87309C36B356F91395B16BA11AF7267E51AC3857EF4CDFD122408011220C095A6C8EFBECDAC21C7F8CA670A220E29C7C7791D1038ABEE33840C789CBF2C322083A1BC6A61C4B0AA151C5B548D7B9EA064AF73FEB570B7391FB453455905FE0D3A20A7A02E7CBB627D642622B06B4B51A227CC806382CF10333C84267C014DE307D0422082B1B3AC05FFA5D6F9884262659F5F98E8E9948EB25EFE8C067DCE1D25A084494A2082B1B3AC05FFA5D6F9884262659F5F98E8E9948EB25EFE8C067DCE1D25A084495220048091BC7DDC283F77BFBF91D73C44DA58C3DF8A9CBC867405D8B7F3DAADA22F5A0800000000000000006220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A20E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8557214000000000000000000000000000000000000000012170A1571756572795F6B65793D71756572795F76616C75651A00227708011A480A20E0B7A0713CCE588BA87309C36B356F91395B16BA11AF7267E51AC3857EF4CDFD122408011220C095A6C8EFBECDAC21C7F8CA670A220E29C7C7791D1038ABEE33840C789CBF2C22290803121427A081B935F841C87EB400A37BFA0E7DDC6897E71A0C08B8A0ADB20610E88ECFD101220100",
			wantErr: false,
		},
		{
			name:    "proto: Data: wiretype end group for non-group",
			args:    "0A0E0A04080B10011206302E33382E361211746573742D636861696E2D553874653735180122480A20E0B7A0713CCE588BA87309C36B356F91395B16BA11AF7267E51AC3857EF4CDFD122408011220C095A6C8EFBECDAC21C7F8CA670A220E29C7C7791D1038ABEE33840C789CBF2C2A0C08FEFD8AA006108293A2FE01327C0A3C0A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A123C0A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A3A7C0A3C0A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A123C0A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A427C0A3C0A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A123C0A1427A081B935F841C87EB400A37BFA0E7DDC6897E712220A20877864F9013C73A40B4F24A9F1371FCDC949C3F046EFDCE2181FE9200F837CF1180A480152330A10088080C00A10FFFFFFFFFFFFFFFFFF01120E08A08D0612040880C60A188080401A0B0A09736563703235366B3122002A0058016220E3B0C44298FC1C149AFBF4C8996FB92427AE41E4649B934CA495991B7852B8556A0800000000000000007001",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := hex.DecodeString(tt.args)
			if err != nil {
				t.Fatalf("Failed to decode hex string: %v", err)
			}
			blk, err := DecodeBlock(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeBlock() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr && blk == nil {
				t.Errorf("DecodeBlock() returned nil block")
			}
		})
	}
}
