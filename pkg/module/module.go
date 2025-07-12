package module

// 删除BaseModule抽象，每个具体module直接实现Module接口
// 按照设计原则：module层就是组成task为图，别做任何抽象