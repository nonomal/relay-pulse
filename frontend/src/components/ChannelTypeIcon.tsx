import { useTranslation } from 'react-i18next';
import { getServiceIconComponent } from './ServiceIcon';

export type ChannelType = 'official' | 'reverse' | 'mixed';

/** Parse O-/R-/M- prefix from channel identifier. Returns null for unrecognized formats. */
export function parseChannelType(channel?: string | null): ChannelType | null {
  if (!channel) return null;
  const prefix = channel.charAt(0);
  if (channel.charAt(1) !== '-') return null;
  if (prefix === 'O') return 'official';
  if (prefix === 'R') return 'reverse';
  if (prefix === 'M') return 'mixed';
  return null;
}

// Lightning bolt (reverse/unofficial)
function ReverseIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <path
        d="M13 2L4.5 13H12l-1 9L19.5 11H12l1-9z"
        className="fill-warning"
      />
    </svg>
  );
}

// Two overlapping cards (mixed sources)
function MixedIcon() {
  return (
    <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" aria-hidden="true">
      <rect x="3" y="6" width="12" height="14" rx="2" className="fill-muted" opacity="0.4" />
      <rect x="9" y="4" width="12" height="14" rx="2" className="fill-muted" />
    </svg>
  );
}

interface ChannelTypeIconProps {
  channel?: string | null;
  serviceType?: string;
}

/** Renders a channel type icon based on O-/R-/M- prefix. O- uses the service's official icon. */
export function ChannelTypeIcon({ channel, serviceType }: ChannelTypeIconProps) {
  const { t } = useTranslation();
  const type = parseChannelType(channel);
  if (!type) return null;

  const label = t(`table.channelType.${type}`);

  let icon: React.JSX.Element;
  if (type === 'official') {
    const ServiceIcon = serviceType ? getServiceIconComponent(serviceType) : null;
    icon = ServiceIcon ? <ServiceIcon className="w-3.5 h-3.5" /> : <ReverseIcon />;
  } else if (type === 'reverse') {
    icon = <ReverseIcon />;
  } else {
    icon = <MixedIcon />;
  }

  return (
    <span className="inline-flex flex-shrink-0" title={label} role="img" aria-label={label}>
      {icon}
    </span>
  );
}
