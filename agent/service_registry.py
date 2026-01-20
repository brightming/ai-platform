"""
AI Platform Service Registry Client
自研镜像使用此客户端进行服务注册和心跳通信
"""

import os
import socket
import time
import threading
from typing import Dict, Any, Optional, Callable
from dataclasses import dataclass, field, asdict
import requests
import psutil


@dataclass
class ServiceCapabilities:
    """服务能力元数据"""
    supported_models: list[str] = field(default_factory=list)
    supported_resolutions: list[str] = field(default_factory=list)
    max_batch_size: int = 4
    supported_formats: list[str] = field(default_factory=lambda: ["png", "jpeg"])
    inference_steps_range: list[int] = field(default_factory=lambda: [10, 100])
    guidance_scale_range: list[float] = field(default_factory=lambda: [1.0, 20.0])
    custom_capabilities: Dict[str, Any] = field(default_factory=dict)


@dataclass
class ResourceSpec:
    """资源规格"""
    gpu_memory: str = "16GB"
    gpu_count: int = 1
    gpu_model: str = "A10"
    cpu_cores: int = 8
    memory: str = "32GB"


@dataclass
class PerformanceSpec:
    """性能规格"""
    estimated_latency_ms: int = 2000
    throughput_per_minute: int = 30
    warmup_time_seconds: int = 60


@dataclass
class RegisterRequest:
    """服务注册请求"""
    service_type: str  # text_to_image, image_editing, etc.
    version: str = "1.0.0"
    hostname: str = ""
    ip_address: str = ""
    port: int = 8080
    capabilities: ServiceCapabilities = None
    resources: ResourceSpec = None
    performance: PerformanceSpec = None

    def __post_init__(self):
        if self.hostname == "":
            self.hostname = socket.gethostname()
        if self.ip_address == "":
            self.ip_address = socket.gethostbyname(self.hostname)


@dataclass
class HeartbeatRequest:
    """心跳请求"""
    service_id: str
    timestamp: str = ""
    current_load: float = 0.0
    queue_size: int = 0
    processed_count: int = 0
    error_count: int = 0
    cpu_utilization: float = 0.0
    gpu_utilization: float = 0.0
    memory_usage: int = 0
    token: str = ""

    def __post_init__(self):
        if self.timestamp == "":
            self.timestamp = time.utcnow().isoformat()


class ServiceRegistryClient:
    """服务注册中心客户端"""

    def __init__(
        self,
        server_url: str = None,
        capabilities: ServiceCapabilities = None,
        resources: ResourceSpec = None,
        performance: PerformanceSpec = None
    ):
        self.server_url = server_url or os.getenv(
            "SERVICE_REGISTRY_URL",
            "http://service-registry.ai-platform.svc.cluster.local:80"
        )
        self.service_id: Optional[str] = None
        self.heartbeat_interval: int = 30
        self.token: str = ""
        self._running = False
        self._thread: Optional[threading.Thread] = None
        self._stop_event = threading.Event()

        # 能力元数据
        self.capabilities = capabilities or ServiceCapabilities()
        self.resources = resources or ResourceSpec()
        self.performance = performance or PerformanceSpec()

        # 状态回调
        self.on_config_update: Optional[Callable] = None
        self.on_shutdown: Optional[Callable] = None

    def register(
        self,
        service_type: str,
        version: str = "1.0.0",
        **kwargs
    ) -> Dict[str, Any]:
        """
        注册服务到控制平面

        Args:
            service_type: 服务类型 (text_to_image, image_editing, etc.)
            version: 服务版本
            **kwargs: 其他参数

        Returns:
            注册响应
        """
        request = RegisterRequest(
            service_type=service_type,
            version=version,
            capabilities=self.capabilities,
            resources=self.resources,
            performance=self.performance,
            **kwargs
        )

        response = requests.post(
            f"{self.server_url}/api/v1/services/register",
            json=asdict(request),
            timeout=10
        )
        response.raise_for_status()

        data = response.json()
        if data.get("code") == 0:
            self.service_id = data["data"]["service_id"]
            self.heartbeat_interval = data["data"]["heartbeat_interval"]
            self.token = data["data"]["token"]

        return data["data"]

    def heartbeat(self, **kwargs) -> Dict[str, Any]:
        """
        发送心跳

        Args:
            **kwargs: 覆盖默认指标

        Returns:
            心跳响应
        """
        if not self.service_id:
            raise RuntimeError("Service not registered")

        # 获取系统指标
        request = HeartbeatRequest(
            service_id=self.service_id,
            current_load=psutil.cpu_percent() / 100.0,
            queue_size=kwargs.get("queue_size", 0),
            processed_count=kwargs.get("processed_count", 0),
            error_count=kwargs.get("error_count", 0),
            cpu_utilization=psutil.cpu_percent(),
            gpu_utilization=self._get_gpu_utilization(),
            memory_usage=psutil.virtual_memory().used,
            token=self.token
        )

        response = requests.post(
            f"{self.server_url}/api/v1/services/heartbeat",
            json=asdict(request),
            timeout=5
        )
        response.raise_for_status()

        data = response.json()
        if data.get("code") == 0:
            heartbeat_data = data["data"]
            # 检查配置更新
            if heartbeat_data.get("config_update"):
                self._handle_config_update(heartbeat_data["config_update"])
            # 检查是否需要关闭
            if heartbeat_data.get("drain_requested"):
                self._handle_drain_request()

        return data["data"]

    def shutdown(self, reason: str = "") -> Dict[str, Any]:
        """
        请求优雅关闭

        Args:
            reason: 关闭原因

        Returns:
            关闭响应
        """
        if not self.service_id:
            raise RuntimeError("Service not registered")

        request = {
            "service_id": self.service_id,
            "reason": reason
        }

        response = requests.post(
            f"{self.server_url}/api/v1/services/shutdown",
            json=request,
            timeout=5
        )
        response.raise_for_status()

        return response.json()["data"]

    def start_heartbeat_loop(self):
        """启动心跳循环"""
        if self._running:
            return

        self._running = True
        self._stop_event.clear()
        self._thread = threading.Thread(target=self._heartbeat_loop, daemon=True)
        self._thread.start()

    def stop_heartbeat_loop(self):
        """停止心跳循环"""
        if not self._running:
            return

        self._running = False
        self._stop_event.set()

        if self._thread:
            self._thread.join(timeout=5)

    def _heartbeat_loop(self):
        """心跳循环"""
        while self._running:
            try:
                self.heartbeat()
            except Exception as e:
                print(f"Heartbeat failed: {e}")

            # 等待下一次心跳，或停止事件
            self._stop_event.wait(self.heartbeat_interval)

    def _get_gpu_utilization(self) -> float:
        """
        获取GPU利用率

        Returns:
            GPU使用率百分比
        """
        try:
            import GPUtil
            gpus = GPUtil.getGPUs()
            if gpus:
                return gpus[0].load * 100
        except ImportError:
            pass
        except Exception:
            pass

        return 0.0

    def _handle_config_update(self, config_update: Dict[str, Any]):
        """处理配置更新"""
        if self.on_config_update:
            self.on_config_update(config_update)
        else:
            print(f"Config update received: {config_update}")

    def _handle_drain_request(self):
        """处理排空请求"""
        if self.on_shutdown:
            self.on_shutdown()
        else:
            print("Drain requested, shutting down gracefully...")
            self.stop_heartbeat_loop()

    def __enter__(self):
        """上下文管理器入口"""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """上下文管理器退出"""
        self.stop_heartbeat_loop()


# 示例用法
if __name__ == "__main__":
    # 创建服务注册客户端
    client = ServiceRegistryClient(
        capabilities=ServiceCapabilities(
            supported_models=["SDXL", "SD1.5"],
            supported_resolutions=["512x512", "1024x1024", "2048x2048"],
            max_batch_size=4
        ),
        resources=ResourceSpec(
            gpu_memory="16GB",
            gpu_count=1,
            gpu_model="A10"
        ),
        performance=PerformanceSpec(
            estimated_latency_ms=2000,
            throughput_per_minute=30
        )
    )

    # 注册服务
    try:
        resp = client.register(service_type="text_to_image")
        print(f"Registered: {resp}")

        # 启动心跳
        client.start_heartbeat_loop()

        # 模拟运行
        for i in range(10):
            print(f"Running... {i}")
            time.sleep(5)

    finally:
        # 关闭
        client.stop_heartbeat_loop()
        client.shutdown("Service stopping")
