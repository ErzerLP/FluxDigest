[Unit]
Description=FluxDigest scheduler service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=__APP_USER__
Group=__APP_GROUP__
WorkingDirectory=__APP_DIR__
EnvironmentFile=__ENV_FILE__
ExecStart=__APP_DIR__/bin/rss-scheduler
Restart=always
RestartSec=5s
TimeoutStopSec=20s
KillMode=process
NoNewPrivileges=true
LimitNOFILE=65535
SyslogIdentifier=fluxdigest-scheduler

[Install]
WantedBy=multi-user.target
