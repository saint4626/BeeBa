const ok2xxOr3xx = `(async function (statusCode, responseTime) {
  const family = Math.floor(statusCode / 100);
  if (family >= 2 && family <= 3) {
    return { status: "UP", latency: responseTime };
  }
  return { status: "DOWN", latency: responseTime };
})`;

const readyzEval = `(async function (statusCode, responseTime, responseRaw) {
  if (statusCode < 200 || statusCode >= 300) {
    return { status: "DOWN", latency: responseTime };
  }

  try {
    const payload = JSON.parse(responseRaw);
    if (payload.status === "ready") {
      return { status: "UP", latency: responseTime };
    }
  } catch (error) {
    return { status: "DOWN", latency: responseTime };
  }

  return { status: "DEGRADED", latency: responseTime };
})`;

const jsonAvailableEval = `(async function (statusCode, responseTime, responseRaw) {
  if (statusCode < 200 || statusCode >= 300) {
    return { status: "DOWN", latency: responseTime };
  }

  try {
    const payload = JSON.parse(responseRaw);
    if (payload.status === "available") {
      return { status: "UP", latency: responseTime };
    }
  } catch (error) {
    return { status: "DOWN", latency: responseTime };
  }

  return { status: "DEGRADED", latency: responseTime };
})`;

const apiTypeData = (url: string, evalBody = ok2xxOr3xx) =>
  JSON.stringify({
    url,
    method: "GET",
    headers: [],
    body: "",
    timeout: 10000,
    eval: evalBody,
    allowSelfSignedCert: false,
  });

const defaultMonitorSettings = {
  uptime_formula_numerator: "up + maintenance",
  uptime_formula_denominator: "up + maintenance + down + degraded",
};

const seedMonitorData = [
  {
    tag: "beeba-web",
    name: "BeeBa Web",
    description: "Astro frontend and public catalog shell.",
    image: "",
    cron: "* * * * *",
    default_status: "UP",
    status: "ACTIVE",
    category_name: "BeeBa",
    monitor_type: "API",
    down_trigger: null,
    degraded_trigger: null,
    type_data: apiTypeData("http://frontend:4321"),
    day_degraded_minimum_count: 1,
    day_down_minimum_count: 1,
    include_degraded_in_downtime: "NO",
    is_hidden: "NO",
    monitor_settings_json: JSON.stringify(defaultMonitorSettings),
  },
  {
    tag: "beeba-api",
    name: "BeeBa API",
    description: "Go/Fiber backend readiness, including configured PostgreSQL and Redis checks.",
    image: "",
    cron: "* * * * *",
    default_status: "UP",
    status: "ACTIVE",
    category_name: "BeeBa",
    monitor_type: "API",
    down_trigger: null,
    degraded_trigger: null,
    type_data: apiTypeData("http://backend:8080/readyz", readyzEval),
    day_degraded_minimum_count: 1,
    day_down_minimum_count: 1,
    include_degraded_in_downtime: "NO",
    is_hidden: "NO",
    monitor_settings_json: JSON.stringify(defaultMonitorSettings),
  },
  {
    tag: "beeba-openapi",
    name: "BeeBa OpenAPI",
    description: "Published OpenAPI document used by the API reference.",
    image: "",
    cron: "* * * * *",
    default_status: "UP",
    status: "ACTIVE",
    category_name: "BeeBa",
    monitor_type: "API",
    down_trigger: null,
    degraded_trigger: null,
    type_data: apiTypeData("http://backend:8080/openapi.yaml"),
    day_degraded_minimum_count: 1,
    day_down_minimum_count: 1,
    include_degraded_in_downtime: "NO",
    is_hidden: "NO",
    monitor_settings_json: JSON.stringify(defaultMonitorSettings),
  },
  {
    tag: "beeba-search",
    name: "BeeBa Search",
    description: "Meilisearch service health for catalog search.",
    image: "",
    cron: "* * * * *",
    default_status: "UP",
    status: "ACTIVE",
    category_name: "BeeBa",
    monitor_type: "API",
    down_trigger: null,
    degraded_trigger: null,
    type_data: apiTypeData("http://meilisearch:7700/health", jsonAvailableEval),
    day_degraded_minimum_count: 1,
    day_down_minimum_count: 1,
    include_degraded_in_downtime: "NO",
    is_hidden: "NO",
    monitor_settings_json: JSON.stringify(defaultMonitorSettings),
  },
  {
    tag: "beeba-storage",
    name: "BeeBa Object Storage",
    description: "MinIO readiness for .bee packages and media assets.",
    image: "",
    cron: "* * * * *",
    default_status: "UP",
    status: "ACTIVE",
    category_name: "BeeBa",
    monitor_type: "API",
    down_trigger: null,
    degraded_trigger: null,
    type_data: apiTypeData("http://minio:9000/minio/health/ready"),
    day_degraded_minimum_count: 1,
    day_down_minimum_count: 1,
    include_degraded_in_downtime: "NO",
    is_hidden: "NO",
    monitor_settings_json: JSON.stringify(defaultMonitorSettings),
  },
];

export default seedMonitorData;
