import pytest

def pytest_addoption(parser):
    parser.addoption(
        "--host",action="store",default="10.2.184.31",help="主机host"
    )
    parser.addoption(
        "--user",action="store",default="root",help="登录用户名"
    )
    parser.addoption(
        "--password",action="store",default="driver",help="登录密码"
    )
    parser.addoption(
        "--CRDVersion",action="store",default="1.0.0-master",help="测试的operator版本"
    )
@pytest.fixture
def host(request):
    return request.config.getoption("--host")

@pytest.fixture
def user(request):
    return request.config.getoption("--user")

@pytest.fixture
def password(request):
    return request.config.getoption("--password")

@pytest.fixture
def CRDVersion(request):
    return request.config.getoption("--CRDVersion")