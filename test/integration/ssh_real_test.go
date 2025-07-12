package integration

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
)

const (
	// SSH测试配置信息
	testSSHHost         = "192.168.31.34"
	testSSHPort         = 2222
	testSSHUser         = "root"
	testSSHPassword     = "rootpassword"
	testSSHPrivateKey   = "/home/mensyli1/workspace/kubexm/test_id_rsa"
	testSudoUser        = "testuser"
	testSudoPassword    = "testpassword"
	testSudoPrivateKey  = "/home/mensyli1/workspace/kubexm/test_id_rsa"
)

// TestSSHRealConnection 测试真实的SSH连接
func TestSSHRealConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过真实SSH连接测试（短模式）")
	}

	// 检查环境变量，允许用户跳过真实连接测试
	if os.Getenv("KUBEXM_SKIP_REAL_SSH_TESTS") == "1" {
		t.Skip("跳过真实SSH连接测试（环境变量设置）")
	}

	ctx := context.Background()

	t.Run("密码认证连接测试", func(t *testing.T) {
		// 创建使用密码认证的连接器
		hostSpec := v1alpha1.Host{
			Name:     "test-ssh-host",
			Type:     "ssh",
			Address:  testSSHHost,
			Port:     testSSHPort,
			User:     testSSHUser,
			Password: testSSHPassword,
		}

		conn := connector.NewHostFromSpec(hostSpec)
		require.NotNil(t, conn, "连接器不应为空")

		// 测试基本连接
		sshConn, ok := conn.(*connector.SSHConnector)
		require.True(t, ok, "应该是SSH连接器类型")

		t.Logf("尝试连接到 %s:%d，用户：%s", testSSHHost, testSSHPort, testSSHUser)

		// 测试OS信息获取
		osInfo, err := sshConn.GetOS(ctx)
		if err != nil {
			t.Logf("连接失败: %v", err)
			t.Skip("无法连接到测试主机，跳过测试")
		}
		require.NoError(t, err, "获取OS信息应该成功")
		assert.NotNil(t, osInfo, "OS信息不应为空")

		t.Logf("检测到OS: %s %s (%s)", osInfo.ID, osInfo.VersionID, osInfo.Arch)
	})

	t.Run("私钥认证连接测试", func(t *testing.T) {
		// 检查私钥文件是否存在
		if _, err := os.Stat(testSSHPrivateKey); os.IsNotExist(err) {
			t.Skipf("私钥文件不存在: %s", testSSHPrivateKey)
		}

		// 创建使用私钥认证的连接器
		hostSpec := v1alpha1.Host{
			Name:       "test-ssh-key-host",
			Type:       "ssh",
			Address:    testSSHHost,
			Port:       testSSHPort,
			User:       testSSHUser,
			PrivateKey: testSSHPrivateKey,
		}

		conn := connector.NewHostFromSpec(hostSpec)
		require.NotNil(t, conn, "连接器不应为空")

		sshConn, ok := conn.(*connector.SSHConnector)
		require.True(t, ok, "应该是SSH连接器类型")

		t.Logf("尝试使用私钥连接到 %s:%d，用户：%s", testSSHHost, testSSHPort, testSSHUser)

		// 测试连接
		osInfo, err := sshConn.GetOS(ctx)
		if err != nil {
			t.Logf("私钥连接失败: %v", err)
			t.Skip("私钥认证失败，可能需要配置SSH密钥")
		}
		require.NoError(t, err, "私钥认证应该成功")
		assert.NotNil(t, osInfo, "OS信息不应为空")
	})

	t.Run("Sudo用户连接测试", func(t *testing.T) {
		// 创建sudo用户连接
		hostSpec := v1alpha1.Host{
			Name:     "test-sudo-host",
			Type:     "ssh",
			Address:  testSSHHost,
			Port:     testSSHPort,
			User:     testSudoUser,
			Password: testSudoPassword,
		}

		conn := connector.NewHostFromSpec(hostSpec)
		require.NotNil(t, conn, "连接器不应为空")

		sshConn, ok := conn.(*connector.SSHConnector)
		require.True(t, ok, "应该是SSH连接器类型")

		t.Logf("尝试以sudo用户连接: %s", testSudoUser)

		// 测试连接
		osInfo, err := sshConn.GetOS(ctx)
		if err != nil {
			t.Logf("Sudo用户连接失败: %v", err)
			t.Skip("Sudo用户连接失败")
		}
		require.NoError(t, err, "Sudo用户连接应该成功")
		assert.NotNil(t, osInfo, "OS信息不应为空")
	})
}

// TestSSHRunnerOperations 测试通过SSH运行器执行操作
func TestSSHRunnerOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过SSH运行器测试（短模式）")
	}

	if os.Getenv("KUBEXM_SKIP_REAL_SSH_TESTS") == "1" {
		t.Skip("跳过真实SSH连接测试（环境变量设置）")
	}

	ctx := context.Background()

	// 创建连接器
	hostSpec := v1alpha1.Host{
		Name:     "test-runner-host",
		Type:     "ssh",
		Address:  testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	}

	conn := connector.NewHostFromSpec(hostSpec)
	require.NotNil(t, conn, "连接器不应为空")

	r := runner.NewRunner()

	// 首先测试连接
	osInfo, err := conn.GetOS(ctx)
	if err != nil {
		t.Skipf("无法连接到测试主机: %v", err)
	}

	t.Run("基本命令执行", func(t *testing.T) {
		// 测试简单命令
		output, err := r.Run(ctx, conn, "echo 'Hello from SSH'", false)
		require.NoError(t, err, "Echo命令应该成功")
		assert.Contains(t, output, "Hello from SSH", "输出应该包含预期文本")

		// 测试主机名获取
		hostname, err := r.Run(ctx, conn, "hostname", false)
		require.NoError(t, err, "获取主机名应该成功")
		assert.NotEmpty(t, hostname, "主机名不应为空")
		t.Logf("远程主机名: %s", hostname)

		// 测试当前用户
		whoami, err := r.Run(ctx, conn, "whoami", false)
		require.NoError(t, err, "whoami命令应该成功")
		assert.Contains(t, whoami, testSSHUser, "应该返回当前用户")
	})

	t.Run("系统信息收集", func(t *testing.T) {
		// 测试facts收集
		facts, err := r.GatherFacts(ctx, conn)
		require.NoError(t, err, "收集facts应该成功")
		require.NotNil(t, facts, "Facts不应为空")

		assert.NotEmpty(t, facts.OS.ID, "OS ID不应为空")
		assert.NotEmpty(t, facts.Hostname, "主机名不应为空")
		assert.Greater(t, facts.TotalCPU, 0, "CPU数量应该大于0")
		assert.Greater(t, facts.TotalMemory, uint64(0), "内存应该大于0")

		t.Logf("远程系统信息:")
		t.Logf("  OS: %s %s", facts.OS.ID, facts.OS.VersionID)
		t.Logf("  主机名: %s", facts.Hostname)
		t.Logf("  CPU: %d核", facts.TotalCPU)
		t.Logf("  内存: %d MB", facts.TotalMemory/(1024*1024))
		t.Logf("  架构: %s", facts.OS.Arch)
	})

	t.Run("文件操作测试", func(t *testing.T) {
		testFile := "/tmp/kubexm-ssh-test.txt"
		testContent := []byte("这是SSH测试文件内容\n时间: " + time.Now().Format(time.RFC3339))

		// 写入文件
		err := r.WriteFile(ctx, conn, testContent, testFile, "0644", false)
		require.NoError(t, err, "写入文件应该成功")

		// 检查文件是否存在
		exists, err := r.Exists(ctx, conn, testFile)
		require.NoError(t, err, "检查文件存在应该成功")
		assert.True(t, exists, "文件应该存在")

		// 读取文件
		readContent, err := r.ReadFile(ctx, conn, testFile)
		require.NoError(t, err, "读取文件应该成功")
		assert.Equal(t, testContent, readContent, "读取的内容应该匹配")

		// 清理测试文件
		err = r.Remove(ctx, conn, testFile, false)
		require.NoError(t, err, "删除文件应该成功")

		// 验证文件已删除
		exists, err = r.Exists(ctx, conn, testFile)
		require.NoError(t, err, "检查文件存在应该成功")
		assert.False(t, exists, "文件应该已删除")
	})

	t.Run("目录操作测试", func(t *testing.T) {
		testDir := "/tmp/kubexm-ssh-test-dir"

		// 创建目录
		err := r.Mkdirp(ctx, conn, testDir, "0755", false)
		require.NoError(t, err, "创建目录应该成功")

		// 检查是否为目录
		isDir, err := r.IsDir(ctx, conn, testDir)
		require.NoError(t, err, "检查目录应该成功")
		assert.True(t, isDir, "应该是目录")

		// 清理测试目录
		err = r.Remove(ctx, conn, testDir, false)
		require.NoError(t, err, "删除目录应该成功")
	})

	t.Run("sudo权限测试", func(t *testing.T) {
		// 测试需要sudo权限的命令
		options := &connector.ExecOptions{
			Sudo:    true,
			Timeout: 30 * time.Second,
		}

		// 尝试读取需要root权限的文件
		stdout, stderr, err := r.RunWithOptions(ctx, conn, "cat /etc/shadow | head -1", options)
		if err != nil {
			t.Logf("Sudo命令失败: %v, stderr: %s", err, string(stderr))
			// 这可能失败，因为需要配置sudo权限
			t.Log("注意: sudo权限可能需要额外配置")
		} else {
			assert.NotEmpty(t, stdout, "应该读取到shadow文件内容")
			t.Log("Sudo权限测试成功")
		}
	})
}

// TestSSHPerformance 测试SSH性能
func TestSSHPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过SSH性能测试（短模式）")
	}

	if os.Getenv("KUBEXM_SKIP_REAL_SSH_TESTS") == "1" {
		t.Skip("跳过真实SSH连接测试（环境变量设置）")
	}

	ctx := context.Background()

	// 创建连接器
	hostSpec := v1alpha1.Host{
		Name:     "test-perf-host",
		Type:     "ssh",
		Address:  testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	}

	conn := connector.NewHostFromSpec(hostSpec)
	require.NotNil(t, conn, "连接器不应为空")

	r := runner.NewRunner()

	// 测试连接
	_, err := conn.GetOS(ctx)
	if err != nil {
		t.Skipf("无法连接到测试主机: %v", err)
	}

	t.Run("连续命令执行性能", func(t *testing.T) {
		numCommands := 10
		start := time.Now()

		for i := 0; i < numCommands; i++ {
			_, err := r.Run(ctx, conn, fmt.Sprintf("echo 'command %d'", i), false)
			require.NoError(t, err, "命令执行应该成功")
		}

		duration := time.Since(start)
		avgDuration := duration / time.Duration(numCommands)

		t.Logf("执行%d个命令耗时: %v", numCommands, duration)
		t.Logf("平均每个命令耗时: %v", avgDuration)

		// 性能基准: 每个命令应该在5秒内完成
		assert.Less(t, avgDuration, 5*time.Second, "平均命令执行时间应该少于5秒")
	})

	t.Run("并发命令执行测试", func(t *testing.T) {
		numGoroutines := 5
		resultChan := make(chan error, numGoroutines)

		start := time.Now()

		for i := 0; i < numGoroutines; i++ {
			go func(index int) {
				_, err := r.Run(ctx, conn, fmt.Sprintf("echo 'concurrent %d'; sleep 1", index), false)
				resultChan <- err
			}(i)
		}

		// 等待所有goroutine完成
		for i := 0; i < numGoroutines; i++ {
			err := <-resultChan
			assert.NoError(t, err, "并发命令应该成功")
		}

		duration := time.Since(start)
		t.Logf("并发执行%d个命令耗时: %v", numGoroutines, duration)

		// 并发执行应该比串行快
		assert.Less(t, duration, 10*time.Second, "并发执行应该在10秒内完成")
	})
}

// TestSSHErrorHandling 测试SSH错误处理
func TestSSHErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过SSH错误处理测试（短模式）")
	}

	if os.Getenv("KUBEXM_SKIP_REAL_SSH_TESTS") == "1" {
		t.Skip("跳过真实SSH连接测试（环境变量设置）")
	}

	ctx := context.Background()
	r := runner.NewRunner()

	t.Run("无效主机连接", func(t *testing.T) {
		hostSpec := v1alpha1.Host{
			Name:     "invalid-host",
			Type:     "ssh",
			Address:  "192.168.255.255", // 不存在的IP
			Port:     22,
			User:     "test",
			Password: "test",
		}

		conn := connector.NewHostFromSpec(hostSpec)
		require.NotNil(t, conn, "连接器不应为空")

		// 设置较短的超时时间
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := conn.GetOS(ctx)
		assert.Error(t, err, "连接到无效主机应该失败")
		t.Logf("预期的连接错误: %v", err)
	})

	t.Run("错误的认证信息", func(t *testing.T) {
		hostSpec := v1alpha1.Host{
			Name:     "wrong-auth-host",
			Type:     "ssh",
			Address:  testSSHHost,
			Port:     testSSHPort,
			User:     testSSHUser,
			Password: "wrongpassword",
		}

		conn := connector.NewHostFromSpec(hostSpec)
		require.NotNil(t, conn, "连接器不应为空")

		_, err := conn.GetOS(ctx)
		assert.Error(t, err, "错误的密码应该导致认证失败")
		t.Logf("预期的认证错误: %v", err)
	})

	t.Run("命令执行失败", func(t *testing.T) {
		hostSpec := v1alpha1.Host{
			Name:     "cmd-fail-host",
			Type:     "ssh",
			Address:  testSSHHost,
			Port:     testSSHPort,
			User:     testSSHUser,
			Password: testSSHPassword,
		}

		conn := connector.NewHostFromSpec(hostSpec)
		require.NotNil(t, conn, "连接器不应为空")

		// 首先确保能连接
		_, err := conn.GetOS(ctx)
		if err != nil {
			t.Skipf("无法连接到测试主机: %v", err)
		}

		// 执行会失败的命令
		_, err = r.Run(ctx, conn, "nonexistent-command", false)
		assert.Error(t, err, "不存在的命令应该失败")

		// 检查是否是CommandError类型
		if cmdErr, ok := err.(*connector.CommandError); ok {
			assert.NotEqual(t, 0, cmdErr.ExitCode, "退出码应该非零")
			t.Logf("命令错误: 退出码=%d, stderr=%s", cmdErr.ExitCode, cmdErr.Stderr)
		}
	})

	t.Run("超时处理", func(t *testing.T) {
		hostSpec := v1alpha1.Host{
			Name:     "timeout-host",
			Type:     "ssh",
			Address:  testSSHHost,
			Port:     testSSHPort,
			User:     testSSHUser,
			Password: testSSHPassword,
		}

		conn := connector.NewHostFromSpec(hostSpec)
		require.NotNil(t, conn, "连接器不应为空")

		// 首先确保能连接
		_, err := conn.GetOS(ctx)
		if err != nil {
			t.Skipf("无法连接到测试主机: %v", err)
		}

		// 执行会超时的命令
		options := &connector.ExecOptions{
			Timeout: 2 * time.Second, // 短超时
		}

		start := time.Now()
		_, _, err = r.RunWithOptions(ctx, conn, "sleep 10", options) // 长时间睡眠
		duration := time.Since(start)

		assert.Error(t, err, "长时间运行的命令应该超时")
		assert.Less(t, duration, 5*time.Second, "超时应该在5秒内触发")
		t.Logf("超时错误: %v, 耗时: %v", err, duration)
	})
}

// BenchmarkSSHOperations SSH操作基准测试
func BenchmarkSSHOperations(b *testing.B) {
	if os.Getenv("KUBEXM_SKIP_REAL_SSH_TESTS") == "1" {
		b.Skip("跳过真实SSH连接测试（环境变量设置）")
	}

	ctx := context.Background()

	hostSpec := v1alpha1.Host{
		Name:     "benchmark-host",
		Type:     "ssh",
		Address:  testSSHHost,
		Port:     testSSHPort,
		User:     testSSHUser,
		Password: testSSHPassword,
	}

	conn := connector.NewHostFromSpec(hostSpec)
	if conn == nil {
		b.Fatal("无法创建连接器")
	}

	// 测试连接
	_, err := conn.GetOS(ctx)
	if err != nil {
		b.Skipf("无法连接到测试主机: %v", err)
	}

	r := runner.NewRunner()

	b.Run("SimpleCommand", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.Run(ctx, conn, "echo test", false)
		}
	})

	b.Run("FileWrite", func(b *testing.B) {
		content := []byte("benchmark test content")
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			testFile := fmt.Sprintf("/tmp/benchmark-test-%d", i)
			_ = r.WriteFile(ctx, conn, content, testFile, "0644", false)
			_ = r.Remove(ctx, conn, testFile, false)
		}
	})

	b.Run("GatherFacts", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = r.GatherFacts(ctx, conn)
		}
	})
}