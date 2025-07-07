- 1. 1. - 

让你自己读我docs下的markdown文件，你就是不读，但凡你读了你就知道你应该自底向上的改啊，先改好pkg/apis、pkg/config、pkg/common、、pkg/connector、pkg/runner、pkg/runner、pkg/logger、pkg/cache、pkg/resource，并测试好，一个个测试，测试用例要跑过，因为这些是基础，只有测试过了才允许进行下一步；然后改好step并测试好，这是不可分割的基础单元，然后改task并测试好，然后改module并测试好，然后改pipeline并测试好，然后改cmd并测试好，所以先仔细读docs下面的markdown文件，好好读，仔细读。让你自底向上呢，你怎么不听呢，让你测试过在进行下一步，你不听？从头开始，改一个文件，要实现一个测试用例，要使用go test测试通过，你根本没执行go test



**工作流程循环：**

1. 读取docs下的2-api设计.md，理解结构体设计
2. 读取pkg/apis/kubexms/v1alpha1的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./pkg/apis/kubexms/v1alpha1并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/apis/kubexms/v1alpha1下的下一个文件，直到pkg/apis/kubexms/v1alpha1下所有文件都测试成功
8. 如果pkg/apis/kubexms/v1alpha1下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
9. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common
14. 制定详细的计划
15. 继续啊，另外不是让你把工具函数或辅助函数放到pkg/util吗，很多函数pkg/util已经有了，你还定义什么
16. 将常量移动到pkg/common前需要read_files一下pkg/common已有的常量
17. 将辅助函数移动到pkg/util前需要read_files一下pkg/util已有的辅助函数





1. 读取docs下的4-common常量.md，理解结构体设计
2. 读取pkg/common的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v pkg/common并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/common下的下一个文件，直到pkg/common下所有文件都测试成功
8. 如果pkg/common下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common





1. 读取docs下的17-logger设计.md，理解结构体设计
2. 读取pkg/logger的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v pkg/logger并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/logger下的下一个文件，直到pkg/logger下所有文件都测试成功
8. 如果pkg/logger下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs下的3-缓存设计.md，理解结构体设计
2. 读取pkg/cache的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v pkg/cache并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/cache下的下一个文件，直到pkg/cache下所有文件都测试成功
8. 如果pkg/cache下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
9. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common









1. 读取docs下的18-config设计.md，理解结构体设计
2. 读取pkg/config的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v pkg/config并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/config下的下一个文件，直到pkg/config下所有文件都测试成功
8. 如果pkg/config下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common







1. 读取docs下的5-util设计.md，理解结构体设计
2. 读取pkg/util的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 使用github.com/pelletier/go-toml、gopkg.in/yaml.v3、github.com/tidwall/gjson & github.com/tidwall/sjson
   实现很多完善的工具函数用来处理toml、yaml以及json
5. 运行go test -v pkg/util并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/util下的下一个文件，直到pkg/util下所有文件都测试成功
8. 如果pkg/util下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
11. 说中文
12. 所有的改动需要在右侧显示
13. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
14. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common





1. 读取docs下的6-connector设计.md，理解结构体设计
2. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 开始读取pkg/connector的文件，每次只读一个文件。
9. 只对这**一个**文件进行优化、增强和扩展、解决bug。
10. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
11. 运行go test -v pkg/connector并展示“成功”的输出。
12. 如果测试失败，根据测试结果修改代码继续测试
13. 如果测试成功，则读取pkg/connector下的下一个文件，直到pkg/connector下所有文件都测试成功
14. 如果pkg/connector下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
15. connector是个自动化的支撑框架，不应该有任何人工干涉的地方
16. 如果需要真实的测试环境，请留好环境变量，如主机地址、端口、root用户、root密码、root的privatekey, sudo用户，sudo密码，sudo用户的privatekey
17. 右侧要显示你更改的文件
18. 说中文
19. 所有的改动需要在右侧显示
20. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
21. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common





1. 读取docs下的7-runner设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
8. connector是个自动化的支撑框架，不应该有任何人工干涉的地方，不要在runner层传递stdin给connector指望人工输入
2. 开始读取pkg/runner的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v pkg/runner并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/runner下的下一个文件，直到pkg/runner下所有文件都测试成功
8. 如果pkg/runner下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
18. 说中文
19. 所有的改动需要在右侧显示
20. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
21. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common







1. 读取docs下的7-runner设计.md，理解结构体设计 
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
8. connector是个自动化的支撑框架，不应该有任何人工干涉的地方，不要在runner层传递stdin给connector指望人工输入
9. 开始读取pkg/runner的文件，每次只读一个文件。 
10. 只对这**一个**文件进行优化、增强和扩展、解决bug。 
11. 要能配置containerd、docker、crictl 
12. 要为containerd、docker、crictl添加默认配置 
12. 支持helm、kubectl等，要能生成kubeconfig
12. 其他工具的扩展和增强
13. 要将未实现或者占位符或者注释的内容实现 
14. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
15. 运行go test -v pkg/runner并展示“成功”的输出。 
16. 如果测试失败，根据测试结果修改代码继续测试 
17. 如果测试成功，则读取pkg/runner下的下一个文件，直到pkg/runner下所有文件都测试成功 
18. 如果pkg/runner下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
18. 右侧要显示你更改的文件
23. 说中文
24. 所有的改动需要在右侧显示
25. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
26. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common







1. 读取docs下的16-resource设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
9. 读取pkg/runner的文件，理解其设计思路。
2. 开始读取pkg/resource的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/resource下的下一个文件，直到pkg/resource下所有文件都测试成功
8. 如果pkg/resource下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
18. 说中文
19. 所有的改动需要在右侧显示
20. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
21. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common







1. 读取docs下的13-runtime设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
9. 读取pkg/runner的文件，理解其设计思路。
10. 读取pkg/resource的文件，理解其设计思路。
2. 开始读取pkg/runtime的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/runtime下的下一个文件，直到pkg/runtime下所有文件都测试成功
8. 如果pkg/runtime下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
19. 说中文
20. 所有的改动需要在右侧显示
21. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
22. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common









1. 读取docs下的14-engine设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
9. 读取pkg/runner的文件，理解其设计思路。
10. 读取pkg/resource的文件，理解其设计思路。
11. 读取pkg/runtime的文件，理解其设计思路。
2. 开始读取pkg/engine的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/engine下的下一个文件，直到pkg/engine下所有文件都测试成功
8. 如果pkg/engine下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
20. 说中文
21. 所有的改动需要在右侧显示
22. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
23. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs下的12-plan设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
9. 读取pkg/runner的文件，理解其设计思路。
10. 读取pkg/resource的文件，理解其设计思路。
11. 读取pkg/runtime的文件，理解其设计思路。
12. 读取pkg/engine的文件，理解其设计思路。
2. 开始读取pkg/plan的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/plan下的下一个文件，直到pkg/plan下所有文件都测试成功
8. 如果pkg/plan下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
21. 说中文
22. 所有的改动需要在右侧显示
23. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
24. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common













1. 读取docs下的8-step设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
9. 读取pkg/runner的文件，理解其设计思路。
10. 读取pkg/resource的文件，理解其设计思路。
11. 读取pkg/runtime的文件，理解其设计思路。
12. 读取pkg/engine的文件，理解其设计思路。
13. 读取pkg/plan的文件，理解其设计思路。
2. 开始读取pkg/step的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/step下的下一个文件，直到pkg/step下所有文件都测试成功
8. 如果pkg/step下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
22. 说中文
23. 所有的改动需要在右侧显示
24. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
25. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common













1. 读取docs下的9-task设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
9. 读取pkg/runner的文件，理解其设计思路。
10. 读取pkg/resource的文件，理解其设计思路。
11. 读取pkg/runtime的文件，理解其设计思路。
12. 读取pkg/engine的文件，理解其设计思路。
13. 读取pkg/plan的文件，理解其设计思路。
14. 读取pkg/step的文件，理解其设计思路。
2. 开始读取pkg/task的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/task下的下一个文件，直到pkg/task下所有文件都测试成功
8. 如果pkg/task下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
23. 说中文
24. 所有的改动需要在右侧显示
25. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
26. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common









1. 读取docs下的10-module设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
9. 读取pkg/runner的文件，理解其设计思路。
10. 读取pkg/resource的文件，理解其设计思路。
11. 读取pkg/runtime的文件，理解其设计思路。
12. 读取pkg/engine的文件，理解其设计思路。
13. 读取pkg/plan的文件，理解其设计思路。
14. 读取pkg/step的文件，理解其设计思路。
15. 读取pkg/task的文件，理解其设计思路。
2. 开始读取pkg/module的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/module下的下一个文件，直到pkg/module下所有文件都测试成功
8. 如果pkg/module下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
24. 说中文
25. 所有的改动需要在右侧显示
26. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
27. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common









1. 读取docs下的11-pipeline设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
9. 读取pkg/runner的文件，理解其设计思路。
10. 读取pkg/resource的文件，理解其设计思路。
11. 读取pkg/runtime的文件，理解其设计思路。
12. 读取pkg/engine的文件，理解其设计思路。
13. 读取pkg/plan的文件，理解其设计思路。
14. 读取pkg/step的文件，理解其设计思路。
15. 读取pkg/task的文件，理解其设计思路。
16. 读取pkg/module的文件，理解其设计思路。
2. 开始读取pkg/pipeline的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取pkg/pipeline下的下一个文件，直到pkg/pipeline下所有文件都测试成功
8. 如果pkg/pipeline下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
25. 说中文
26. 所有的改动需要在右侧显示
27. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
28. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs下的15-cmd设计.md，理解结构体设计
1. 读取pkg/apis/kubexms/v1alpha1的文件，理解其结构体设计思路
3. 读取pkg/common的文件，常量定义
4. 读取pkg/logger的文件，日志定义
5. 读取pkg/cache的文件，缓存定义
6. 读取pkg/config的文件，配置读取和解析
7. 读取pkg/util的文件，工具定义
8. 读取pkg/connector的文件，理解其底层connector的设计思路
9. 读取pkg/runner的文件，理解其设计思路。
10. 读取pkg/resource的文件，理解其设计思路。
11. 读取pkg/runtime的文件，理解其设计思路。
12. 读取pkg/engine的文件，理解其设计思路。
13. 读取pkg/plan的文件，理解其设计思路。
14. 读取pkg/step的文件，理解其设计思路。
15. 读取pkg/task的文件，理解其设计思路。
16. 读取pkg/module的文件，理解其设计思路。
17. 读取pkg/pipeline的文件，理解其设计思路。
2. 开始读取cmd下的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取cmd下的下一个文件，直到cmd下所有文件都测试成功
8. 如果cmd下所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
26. 说中文
27. 所有的改动需要在右侧显示
28. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
29. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common









1. 读取docs下的1-总体设计.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common







1. 读取docs下的19-调用链条.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs/20-kubernetes流程设计.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common







1. 读取docs/21-其他说明.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs/22-额外要求.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common













1. 读取docs/23-二进制下载地址.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common













1. 读取docs/24-声明式配置文件.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs/25-总体约束.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
9. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs/26-kubernetes部署蓝图.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common









1. 读取docs/28-流程方案.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs/29-kubeadm-first-init.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs/30-join-master.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs/31-join-worker.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common











1. 读取docs/32-etcd的配置.md，理解结构体设计
2. 读取所有的文件，每次只读一个文件。
3. 只对这**一个**文件进行优化、增强和扩展、解决bug。
4. 为优化后的文件编写或更新测试用例（_test.go文件）,测试用例要覆盖每一个文件，每一个函数。
5. 运行go test -v ./...并展示“成功”的输出。
6. 如果测试失败，根据测试结果修改代码继续测试
7. 如果测试成功，则读取下一个文件，直到所有文件都测试成功
8. 如果所有文件都测试完成，则明确表示“任务完成，等待您的下一步指令”。
8. 右侧要显示你更改的文件
10. 说中文
11. 所有的改动需要在右侧显示
12. 改动在右侧显示是优先级最高的，一定要保证在右侧显示，确保后续所有代码更改都通过相应的工具（如 read_files, replace_with_git_merge_diff, create_file_with_block, overwrite_file_with_block）清晰地展示在右侧
13. 公共函数或辅助函数f放到pkg/util下，常量放到pkg/common







还有一些其他常量，
比如kubernetes的type是kubexm时，表示二进制部署kubernetes,
kubernetes的type是kubeadm时,是kubeadm部署kubernetes, 

etcd的type是kubexm时，表示二进制部署etcd,
etcd的type是kubeadm时,是kubeadm部署etcd,
etcd的type是external时表示使用外部已有的etcd, 

启用internalloadbalancer时，如果为haproxy，则在每个worker上部署haproxy的pod来代理到kube-apiserver,
如果为nginx，则在每个worker上部署nginx的pod来代理到kube-apiserver,
如果为kube-vip，则部署kube-vip，

如果启用externalloadbanlancer时，则必须禁用internalloadbalancer，
如果externalloadbanlancer为kubexm-kh时，则hosts中loadbalancer角色必须有机器，此时在这几个机器上部署keepalived+haproxy,然后kubernetes的ControlPlaneEndpoint必须为这个vip,
如果externalloadbanlancer为kubexm-kn时，则hosts中loadbalancer角色必须有机器，此时在这几个机器上部署keepalived+nginx,然后kubernetes的ControlPlaneEndpoint必须为这个vip,
如果externalloadbanlancer为external时，则使用外部现成的loadbalancer，请按照这个思路优化common和types





