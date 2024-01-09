import pytest


def pytest_addoption(parser):
    parser.addoption("--force-aws-upload", action="store_true", default=False,
                     help=("Force AWS upload when building AMI, failing if credentials are not set. "
                           "If not set, the upload will be performed only when credentials are available."))


@pytest.fixture(name="force_aws_upload", scope="session")
def force_aws_upload_fixture(request):
    return request.config.getoption("--force-aws-upload")
