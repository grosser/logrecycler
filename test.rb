# integration tests that test exit codes and piping/commands since that is hard in go
#
# run individual tests by passing their name as regex
# ruby test.rb -n '/returns the commands exit code/'

require "minitest/autorun"
require "tmpdir"
require "timeout"
require "benchmark"
require "open3"

def sh(command, expected_exit: 0, timeout: 1)
  result = Timeout.timeout(timeout, RuntimeError, "Timed out when running #{command}") { `#{command} 2>&1` }
  if $?.exitstatus != expected_exit
    raise "Command exited with #{$?.exitstatus}, not the expected #{expected_exit}:\n#{command}\n#{result}"
  end
  result
end

sh "go build .", timeout: 10

describe "logrecycler" do
  standard_boot_time = 0.5 # basic execution takes 0.2 locally, so we need to wait longer to make sure program started

  def with_config(content)
    Dir.mktmpdir do |dir|
      Dir.chdir(dir) do
        File.write("logrecycler.yaml", content)
        yield
      end
    end
  end

  def call(extra, pipe: "", **args)
    command = "#{full_path} #{extra}"
    command = "echo #{pipe} | #{command}" if pipe
    _(sh(command, **args))
  end

  let(:full_path) { File.expand_path("./logrecycler", __dir__) }

  it "can show help" do
    call("--help").must_include "logrecycler"
  end

  it "can show version" do
    call("--version").must_equal "master\n"
  end

  it "fails fast with unknown arguments" do
    out = call("--wut", expected_exit: 2)
    out.must_include "flag provided"
    out.wont_include "no such file or directory" # did not try to read file
  end

  it "fails nicely with no file" do
    with_config "" do
      File.unlink "logrecycler.yaml"
      call("", expected_exit: 2).must_equal "Error: open logrecycler.yaml: no such file or directory\n"
    end
  end

  it "shows location when failing on bad regex in preprocess" do
    with_config "preprocess: '((((WUT'" do
      call("", expected_exit: 1).must_equal "Error: regular expression from preprocess: error parsing regexp: missing closing ): `((((WUT`"
    end
  end

  it "shows location when failing on bad regex in pattern" do
    with_config "patterns:\n- regex: '((((WUT'" do
      call("", expected_exit: 1).must_equal "Error: regular expression from patterns[0].regex: error parsing regexp: missing closing ): `((((WUT`"
    end
  end

  it "fails when neither passing stdin nor args" do
    with_config "" do
      call("", pipe: nil, expected_exit: 2).must_include "pipe logs"
    end
  end

  describe "commands" do
    it "fails when piping with command" do
      with_config "" do
        call("-- echo 12", expected_exit: 2).must_include "logrecycler"
      end
    end

    it "can process a command that finishes" do
      with_config "" do
        call("-- echo 12", pipe: nil).must_equal "{\"message\":\"12\"}\n"
      end
    end

    it "can stream a command that streams" do
      wait = standard_boot_time
      with_config "" do
        duration = Benchmark.realtime do
          call("-- sh -c 'echo 1; sleep #{wait};echo 2'", pipe: nil).must_equal "{\"message\":\"1\"}\n{\"message\":\"2\"}\n"
        end
        _(duration).must_be :>=, wait
        _(duration).must_be :<=, wait * 2 # make sure it does not just always wait 1s
      end
    end

    it "outouts to stdout / stderr as it comes in" do
      out, err, status = with_config "" do
        Open3.capture3('echo "OUT"; echo "ERR" >&2')
      end
      _(out).must_equal "OUT\n"
      _(err).must_equal "ERR\n"
    end

    it "returns the commands exit code" do
      with_config "" do
        call("-- sh -c 'exit 13'", pipe: nil, expected_exit: 13).must_equal ""
      end
    end

    it "fails when command fails" do
      with_config "" do
        call("-- wuuut", pipe: nil, expected_exit: 2).must_include "executable file not found"
      end
    end

    it "stops when command is killed" do
      with_config "" do
        Thread.new { sleep standard_boot_time; sh("pkill -f '^sleep 999'") }
        call("-- sleep 999", pipe: nil, expected_exit: 255).must_equal ""
      end
    end

    it "does not leave command running when getting signaled" do
      time = 5
      check_ps = ->(size) do
        ps = sh("ps -ef | grep '[s]leep #{time}'", expected_exit: size == 0 ? 1 : 0)
        _(ps.split("\n").size).must_equal size, ps
      end

      with_config "" do
        Thread.new do
          sleep standard_boot_time
          check_ps.(3)
          sh "pkill -f 'logrecycler -- sleep #{time}'"
        end
        duration = Benchmark.realtime do
          call("-- sleep #{time}", pipe: nil, expected_exit: nil).must_equal ""
        end
        _(duration).must_be :>=, standard_boot_time
        check_ps.(0)
      end
    end
  end
end
