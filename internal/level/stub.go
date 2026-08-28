//go:build !linux && !windows

package level

type stubMonitor struct{}

func newMonitor(DeviceResolver) Monitor { return stubMonitor{} }

func (stubMonitor) Available() bool { return false }

func (stubMonitor) SetStreamOptions(int, int) {}

func (stubMonitor) StartSystem(string) error { return nil }

func (stubMonitor) StartMic(string) error { return nil }

func (stubMonitor) StopSystem() error { return nil }

func (stubMonitor) StopMic() error { return nil }

func (stubMonitor) Read() LevelSnapshot { return LevelSnapshot{} }

func (stubMonitor) Close() error { return nil }

func (stubMonitor) LiveWhenIdle() bool { return false }
