# -*- mode: Python -*-

envsubst_cmd = "envsubst"
tools_bin = "./hack/tools/bin"

#Add tools to path
os.putenv('PATH', os.getenv('PATH') + ':' + tools_bin)

update_settings(k8s_upsert_timeout_secs=60)  # on first tilt up, often can take longer than 30 seconds

# set defaults
settings = {
    "allowed_contexts": [
        "kind-shopkeeper"
    ],
    "deploy_cert_manager": True,
    "preload_images_for_kind": True,
    "kind_cluster_name": "shopkeeper",
    "cert_manager_version": "v1.1.0",
    "kubernetes_version": "v1.22.3",
}

keys = []

# global settings
settings.update(read_json(
    "tilt-settings.json",
    default = {},
))

def validate_auth():
    substitutions = settings.get("kustomize_substitutions", {})
    missing = [k for k in keys if k not in substitutions]
    if missing:
        fail("missing kustomize_substitutions keys {} in tilt-settings.json".format(missing))

tilt_helper_dockerfile_header = """
# Tilt image
FROM golang:1.18 as tilt-helper
# Support live reloading with Tilt
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh
"""

tilt_dockerfile_header = """
FROM gcr.io/distroless/base:debug as tilt
WORKDIR /
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY manager .
"""

def kustomizesub(folder):
    yaml = local('hack/kustomize-sub.sh {}'.format(folder), quiet=True)
    return yaml

def shopkeeper():
    # Apply the kustomized yaml for this provider
    substitutions = settings.get("kustomize_substitutions", {})
    os.environ.update(substitutions)

    yaml = str(kustomizesub("./config/default"))

    # Set up a local_resource build of the provider's manager binary.
    local_resource(
        "manager",
        cmd = 'mkdir -p .tiltbuild;CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags \'-extldflags "-static"\' -o .tiltbuild/manager cmd/main.go',
        deps = ["api", "config", "controllers", "exp", "feature", "pkg", "go.mod", "go.sum", "main.go"],
    )

    dockerfile_contents = "\n".join([
        tilt_helper_dockerfile_header,
        tilt_dockerfile_header,
    ])

    entrypoint = ["sh", "/start.sh", "/manager"]
    extra_args = settings.get("extra_args")
    if extra_args:
        entrypoint.extend(extra_args)

    # Set up an image build for the provider. The live update configuration syncs the output from the local_resource
    # build into the container.
    docker_build(
        ref = "shopkeeper-controller",
        context = "./.tiltbuild/",
        dockerfile_contents = dockerfile_contents,
        target = "tilt",
        entrypoint = entrypoint,
        only = "manager",
        live_update = [
            sync(".tiltbuild/manager", "/manager"),
            run("sh /restart.sh"),
        ],
        ignore = ["templates"],
    )

    k8s_yaml(blob(yaml))

##############################
# Actual work happens here
##############################

validate_auth()

shopkeeper()
