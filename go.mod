module github.com/outblocks/outblocks-cli

go 1.17

require (
	github.com/AlecAivazis/survey/v2 v2.3.2
	github.com/Masterminds/semver v1.5.0
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/Masterminds/vcs v1.13.1
	github.com/docker/docker v20.10.10+incompatible
	github.com/enescakir/emoji v1.0.0
	github.com/go-ozzo/ozzo-validation/v4 v4.3.0
	github.com/goccy/go-yaml v1.8.9
	github.com/google/go-github/v35 v35.3.0
	github.com/gookit/color v1.4.2
	github.com/mholt/archiver/v3 v3.5.1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/otiai10/copy v1.6.0
	github.com/outblocks/outblocks-plugin-go v0.0.0-20211215142113-b2c263826682
	github.com/pterm/pterm v0.12.33
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.9.0
	github.com/txn2/txeh v1.3.0
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	google.golang.org/grpc v1.42.0
	google.golang.org/protobuf v1.27.1
)

require (
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/andybalholm/brotli v1.0.1 // indirect
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d // indirect
	github.com/atomicgo/cursor v0.0.1 // indirect
	github.com/containerd/containerd v1.5.5 // indirect
	github.com/creasty/defaults v1.5.2 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dsnet/compress v0.0.2-0.20210315054119-f66993602bf5 // indirect
	github.com/fatih/color v1.10.0 // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-playground/validator/v10 v10.6.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.3 // indirect
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/klauspost/compress v1.11.13 // indirect
	github.com/klauspost/pgzip v1.2.5 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/nwaples/rardecode v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/pierrec/lz4/v4 v4.1.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/ulikunitz/xz v0.5.9 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/xo/terminfo v0.0.0-20210125001918-ca9a967f8778 // indirect
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5 // indirect
	golang.org/x/net v0.0.0-20210503060351-7fd8e65b6420 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/sys v0.0.0-20211013075003-97ac67df715c // indirect
	golang.org/x/text v0.3.6 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210828152312-66f60bf46e71 // indirect
	gopkg.in/ini.v1 v1.63.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)

replace github.com/goccy/go-yaml => github.com/23doors/go-yaml v1.8.10-0.20210807205153-79a9d3b2c172

// replace github.com/goccy/go-yaml => ../go-yaml

// replace github.com/outblocks/outblocks-plugin-go => ../outblocks-plugin-go
