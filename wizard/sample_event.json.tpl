{
  "orgName": "{{ .OrgName }}",
  "event": "alert",
  "imageUrl": "https://mackerel.io/embed/public/.../....png",
  "memo": "memo....",
  "host": {
    "id": "22D4...",
    "name": "app01",
    "url": "https://mackerel.io/orgs/.../hosts/...",
    "type": "unknown",
    "status": "working",
    "memo": "",
    "isRetired": false,
    "roles": [
      {
        "fullname": "Service: Role",
        "serviceName": "Service",
        "serviceUrl": "https://mackerel.io/orgs/.../services/...",
        "roleName": "ALB",
        "roleUrl": "https://mackerel.io/orgs/.../services/..."
      }
    ]
  },
  "alert": {
    "openedAt": {{ .OpenedAt }},
    "closedAt": {{ .ClosedAt }},
    "createdAt": {{ .CreatedAt }},
    "criticalThreshold": 1.9588528112516932,
    "duration": 5,
    "isOpen": false,
    "metricLabel": "MetricName",
    "metricValue": {{ .MetricValue }},
    "monitorName": "{{ .MonitorName }}",
    "monitorOperator": ">",
    "status": "OK",
    "trigger": "monitor",
    "id": "{{ .AlertID }}",
    "url": "https://mackerel.io/orgs/{{ .OrgName }}/alerts/{{ .AlertID }}",
    "warningThreshold": 0
  }
}
