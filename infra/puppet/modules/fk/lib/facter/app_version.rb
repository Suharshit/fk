# Which app version is deployed (from the "current" symlink target).
Facter.add(:app_version) do
  setcode do
    link = '/opt/fk/current'
    File.symlink?(link) ? File.basename(File.readlink(link)).sub(/^fk-/, '') : nil
  end
end
